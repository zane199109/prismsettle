package demo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/zane/web3-offchain/model"
)

// Script step: who acts, what on-chain action, and the resulting job state.
type scriptStep struct {
	Role   string // buyer / provider / evaluator / system
	Action string // create_and_fund / grab / submit / reject / dispute / resolve / execute / wait
	State  string // job state label shown with the message
}

// demoScript is the fixed scene: provider grabs + submits v1, buyer rejects
// (deposit on first reject), provider resubmits v2, buyer rejects again (no
// deposit), provider disputes, evaluator rules provider wins, escrow settles.
// NOTE: create + fund is executed by the orchestrator before the loop (step
// 0), so it is NOT part of this script.
var demoScript = []scriptStep{
	{Role: "provider", Action: "grab", State: "assigned"},
	{Role: "provider", Action: "submit", State: "submitted"}, // v1
	{Role: "buyer", Action: "reject", State: "submitted"},    // #1 (deposit)
	{Role: "provider", Action: "submit", State: "submitted"}, // v2 resubmit
	{Role: "buyer", Action: "reject", State: "submitted"},    // #2 (no deposit)
	{Role: "provider", Action: "dispute", State: "disputed"},
	{Role: "evaluator", Action: "resolve", State: "resolved"}, // ruling=2
	{Role: "system", Action: "wait", State: "resolved"},       // announcement period
	{Role: "system", Action: "execute", State: "executed"},
}

// directScript is the no-arbitration scene: provider wins the grab, delivers,
// and the buyer accepts directly — escrow settles without a dispute.
var directScript = []scriptStep{
	{Role: "provider", Action: "grab", State: "assigned"},
	{Role: "provider", Action: "submit", State: "submitted"}, // v1
	{Role: "buyer", Action: "complete", State: "completed"},  // direct acceptance
}

// SessionParams is the user-facing request to start a demo.
type SessionParams struct {
	Title         string
	Description   string
	Amount        *big.Int // token units (6 decimals for USDC)
	Token         string   // token contract address
	ProviderAgent string   // senior / junior / rookie
	Scenario      string   // arbitration (default) | direct
	MaxRejects    int      // unused now (script fixed at 2); kept for future
}

// Orchestrator drives a demo session: LLM speech → on-chain action → message.
type Orchestrator struct {
	repo    *SessionRepo
	actions *Actions
	llm     *LLMClient

	// Agent IDs for the demo (provider wallet + evaluator).
	buyerAgentID     *big.Int
	providerAgentID  *big.Int
	evaluatorAgentID *big.Int

	stepDelay          time.Duration
	announcementWait   time.Duration // wait before executing arbitration result

	mu     sync.Mutex
	active bool
}

func NewOrchestrator(
	repo *SessionRepo,
	actions *Actions,
	llm *LLMClient,
	buyerAgentID, providerAgentID, evaluatorAgentID *big.Int,
	stepDelay time.Duration,
	announcementWait time.Duration,
) *Orchestrator {
	if stepDelay <= 0 {
		stepDelay = 1200 * time.Millisecond
	}
	if announcementWait <= 0 {
		announcementWait = 65 * time.Second // legacy default
	}
	return &Orchestrator{
		repo:             repo,
		actions:          actions,
		llm:              llm,
		buyerAgentID:     buyerAgentID,
		providerAgentID:  providerAgentID,
		evaluatorAgentID: evaluatorAgentID,
		stepDelay:        stepDelay,
		announcementWait: announcementWait,
	}
}

// Start begins a new session and runs the script in a background goroutine.
func (o *Orchestrator) Start(ctx context.Context, p SessionParams) (*model.DemoSession, error) {
	o.mu.Lock()
	if o.active {
		o.mu.Unlock()
		return nil, fmt.Errorf("a demo session is already running (serial execution)")
	}
	o.active = true
	o.mu.Unlock()

	id, err := randID()
	if err != nil {
		return nil, err
	}
	if p.Token == "" {
		// Default to the configured payment token when the client omits it.
		p.Token = o.actions.TokenAddr.Hex()
	}
	if p.Scenario == "" {
		p.Scenario = "arbitration"
	}
	sess := &model.DemoSession{
		ID:            id,
		Title:         p.Title,
		Description:   p.Description,
		Amount:        p.Amount.String(),
		Token:         p.Token,
		ProviderAgent: p.ProviderAgent,
		Scenario:      p.Scenario,
		State:         "created",
		MaxRejects:    p.MaxRejects,
	}
	if err := o.repo.CreateSession(ctx, sess); err != nil {
		o.mu.Lock()
		o.active = false
		o.mu.Unlock()
		return nil, err
	}
	go o.run(sess)
	return sess, nil
}

func (o *Orchestrator) GetSession(ctx context.Context, id string) (*model.DemoSession, error) {
	return o.repo.GetSession(ctx, id)
}

// GetByJobID returns the most recent demo session that created the given job
// (nil when the job was never demo-driven) — powers the history view.
func (o *Orchestrator) GetByJobID(ctx context.Context, jobID string) (*model.DemoSession, error) {
	return o.repo.GetByJobID(ctx, jobID)
}

func (o *Orchestrator) ListMessages(ctx context.Context, id string) ([]model.DemoMessage, error) {
	return o.repo.ListMessages(ctx, id)
}

// run executes the script serially. Any failure marks the session failed.
func (o *Orchestrator) run(sess *model.DemoSession) {
	defer func() {
		o.mu.Lock()
		o.active = false
		o.mu.Unlock()
	}()
	ctx := context.Background()
	amount := mustBig(sess.Amount)

	// Step 0: create + fund (no LLM needed for the tx itself; message is
	// system-generated, then the script continues).
	sess.JobID = ""
	msg := &model.DemoMessage{
		SessionID: sess.ID,
		Step:      0,
		Role:      "system",
		Content:   fmt.Sprintf("演示开始：任务「%s」创建并托管 %s（%s）", sess.Title, formatAmount(amount), shortAddr(sess.Token)),
		Action:    "create_and_fund",
		State:     "created",
	}
	_ = o.repo.AppendMessage(ctx, msg)

	// Top up mock funds (escrow + deposits) for all demo roles.
	if err := o.actions.EnsureFunds(ctx, amount); err != nil {
		o.fail(ctx, sess, 0, "ensure_funds", err)
		return
	}

	jobID, _, err := o.actions.CreateAndFund(ctx, o.buyerAgentID, uint64(time.Now().Unix())+3600, new(big.Int).SetUint64(500_000_000_000_000_000), amount)
	if err != nil {
		o.fail(ctx, sess, 0, "create_and_fund", err)
		return
	}
	sess.JobID = fmt.Sprintf("0x%064x", jobID)
	_ = o.repo.UpdateSession(ctx, sess)
	o.repo.AppendMessage(ctx, &model.DemoMessage{
		SessionID: sess.ID, Step: 0, Role: "system",
		Content:  fmt.Sprintf("任务已创建，jobId %s…，5 USDC 已托管", shortID(jobID)),
		Action:   "create_and_fund", State: "funded",
	})

	for i, step := range demoScript {
		if sess.Scenario == "direct" {
			if i >= len(directScript) {
				break
			}
			step = directScript[i]
		} else if i >= len(demoScript) {
			break
		}
		o.repo.AppendMessage(ctx, &model.DemoMessage{
			SessionID: sess.ID, Step: i + 1, Role: "system",
			Content: fmt.Sprintf("【%s】%s…", step.Role, actionLabel(step.Action)),
			Action:  "system_note", State: step.State,
		})
		if err := o.executeStep(ctx, sess, i+1, step); err != nil {
			o.fail(ctx, sess, i+1, step.Action, err)
			return
		}
		// Persist the live state after every step so the front-end Status
		// Tracker follows the flow in real time (not only at the end).
		sess.State = step.State
		_ = o.repo.UpdateSession(ctx, sess)
		time.Sleep(o.stepDelay)
	}

	now := time.Now()
	sess.State = "executed"
	sess.FinishedAt = &now
	sess.Result = fmt.Sprintf(`{"job_id":"%s","outcome":"provider_wins","amount":"%s"}`, sess.JobID, sess.Amount)
	_ = o.repo.UpdateSession(ctx, sess)
	finishMsg := "🎉 演示完成：仲裁裁定 provider 胜，托管资金全额结算给 provider。"
	if sess.Scenario == "direct" {
		finishMsg = "🎉 演示完成：买家验收通过，托管资金全额结算给 provider。"
	}
	o.repo.AppendMessage(ctx, &model.DemoMessage{
		SessionID: sess.ID, Step: len(demoScript) + 1, Role: "system",
		Content: finishMsg,
		Action:  "finish", State: "executed",
	})
}

// executeStep runs one script step: LLM speech (except system steps) + action.
func (o *Orchestrator) executeStep(ctx context.Context, sess *model.DemoSession, stepNo int, step scriptStep) error {
	switch step.Action {
	case "grab":
		// Grab competition — three real outcomes:
		//   1. junior: mismatched agent ID → ownership check reverts (not eligible)
		//   2. senior: correct agent ID → wins the race
		//   3. rookie: correct agent ID but the job is already taken → slow
		o.msg(ctx, sess, stepNo, step, "抢单竞争开始：3 个审计 Agent 同时竞争接单…", "")
		// 1) junior loses on eligibility.
		if _, err := o.actions.GrabCompeting(ctx, mustBig(sess.JobID), o.actions.RookieAgentID(), o.actions.JuniorAuth()); err != nil {
			o.repo.AppendMessage(ctx, &model.DemoMessage{
				SessionID: sess.ID, Step: stepNo, Role: "system",
				Content: "❌ junior 抢单失败：不符合接单资格（归属校验不通过）",
				Action:  "grab_failed", State: "assigned",
			})
		}
		// 2) senior wins.
		reply, err := o.speak(ctx, step.Role, "接单说明", sess, "任务已托管，你准备接单。")
		if err != nil {
			reply = fallbackReply(step.Role, "我来接单。声誉符合要求。")
		}
		tx, err := o.actions.Grab(ctx, mustBig(sess.JobID), o.providerAgentID)
		if err != nil {
			return err
		}
		if err := o.msg(ctx, sess, stepNo, step, reply.Content, tx); err != nil {
			return err
		}
		// 3) rookie is too slow — the job is already assigned.
		if _, err := o.actions.GrabCompeting(ctx, mustBig(sess.JobID), o.actions.RookieAgentID(), o.actions.RookieAuth()); err != nil {
			o.repo.AppendMessage(ctx, &model.DemoMessage{
				SessionID: sess.ID, Step: stepNo, Role: "system",
				Content: "❌ rookie 抢单失败：动作慢了，任务已被其他 agent 抢走",
				Action:  "grab_failed", State: "assigned",
			})
		}
		return nil

	case "submit":
		reply, err := o.speak(ctx, step.Role, "交付物说明", sess, "提交本次交付物（审计报告），说明覆盖内容。")
		if err != nil {
			reply = fallbackReply(step.Role, "提交审计报告：覆盖重入、权限控制与 gas 优化建议。")
		}
		// The deliverable hash is computed over the FULL report when the model
		// produced one (falling back to the bubble text) — the hash on-chain
		// therefore matches the viewable deliverable.
		deliverable := reply.Report
		if deliverable == "" {
			deliverable = reply.Content
		}
		dh := keccak32(deliverable)
		ph := keccak32(deliverable + ":proof")
		tx, err := o.actions.Submit(ctx, mustBig(sess.JobID), dh, ph)
		if err != nil {
			return err
		}
		return o.repo.AppendMessage(ctx, &model.DemoMessage{
			SessionID: sess.ID, Step: stepNo, Role: step.Role,
			Content: reply.Content, Report: reply.Report,
			DeliverableHash: fmt.Sprintf("0x%x", dh),
			Action:          step.Action, TxHash: tx, State: step.State,
		})

	case "reject":
		reply, err := o.speak(ctx, step.Role, "打回意见", sess, "你对交付物不满意，给出专业具体的打回意见（指出缺陷）。")
		if err != nil {
			reply = fallbackReply(step.Role, "审计报告缺少重入漏洞的边界测试用例，请补充后重交。")
		}
		reason := keccak32(reply.Reason)
		tx, err := o.actions.Reject(ctx, mustBig(sess.JobID), reason)
		if err != nil {
			return err
		}
		content := reply.Content
		if reply.Reason != "" && reply.Reason != content {
			content = fmt.Sprintf("%s（理由：%s）", reply.Content, reply.Reason)
		}
		return o.msg(ctx, sess, stepNo, step, content, tx)

	case "complete":
		reply, err := o.speak(ctx, step.Role, "验收通过", sess, "交付物符合任务要求，你验收通过并直接完成结算。")
		if err != nil {
			reply = fallbackReply("buyer", "交付物覆盖所有要求，验收通过，直接结算。")
		}
		// Buyer rates the deliverable (0.9e18) and the escrow settles to the
		// provider — no dispute, no announcement period.
		tx, err := o.actions.Complete(ctx, mustBig(sess.JobID), new(big.Int).SetUint64(900_000_000_000_000_000))
		if err != nil {
			return err
		}
		return o.msg(ctx, sess, stepNo, step, reply.Content, tx)

	case "dispute":
		reply, err := o.speak(ctx, step.Role, "仲裁申请", sess, "你已两次被无充分理由打回，交付物符合规格——提起仲裁。")
		if err != nil {
			reply = fallbackReply(step.Role, "交付物已符合规格，买家打回缺乏依据——我提起仲裁。")
		}
		reason := keccak32(reply.Reason)
		tx, err := o.actions.Dispute(ctx, mustBig(sess.JobID), reason)
		if err != nil {
			return err
		}
		return o.msg(ctx, sess, stepNo, step, reply.Content, tx)

	case "resolve":
		reply, err := o.speak(ctx, step.Role, "仲裁裁定", sess, "你是仲裁方。核对规格与交付物后裁定（本次裁定 provider 胜）。")
		if err != nil {
			reply = fallbackReply(step.Role, "核对交付物与任务规格：重入/权限/边界用例均已覆盖，买家打回缺乏依据，裁定 provider 胜。")
		}
		tx, err := o.actions.Resolve(ctx, mustBig(sess.JobID), 2)
		if err != nil {
			return err
		}
		return o.msg(ctx, sess, stepNo, step, reply.Content, tx)

	case "wait":
		o.msg(ctx, sess, stepNo, step, "裁定已发布，进入公告期（期间任何一方可复核）…", "")
		select {
		case <-time.After(o.announcementWait):
			return o.msg(ctx, sess, stepNo, step, "公告期结束，开始结算。", "")
		case <-ctx.Done():
			return ctx.Err()
		}

	case "execute":
		tx, err := o.actions.Execute(ctx, mustBig(sess.JobID))
		if err != nil {
			return err
		}
		return o.msg(ctx, sess, stepNo, step, "结算完成：托管资金 100% 结算给 provider。", tx)

	default:
		return fmt.Errorf("unknown script action %q", step.Action)
	}
}

// speak asks the LLM for the role's message.
func (o *Orchestrator) speak(ctx context.Context, role, task string, sess *model.DemoSession, hint string) (*LLMReply, error) {
	return o.llm.Speak(ctx, role, systemPrompt(role), contextPrompt(sess, task, hint))
}

// msg appends a chat message.
func (o *Orchestrator) msg(ctx context.Context, sess *model.DemoSession, stepNo int, step scriptStep, content, txHash string) error {
	return o.repo.AppendMessage(ctx, &model.DemoMessage{
		SessionID: sess.ID, Step: stepNo, Role: step.Role,
		Content: content, Action: step.Action, TxHash: txHash, State: step.State,
	})
}

func (o *Orchestrator) fail(ctx context.Context, sess *model.DemoSession, stepNo int, action string, err error) {
	o.repo.AppendMessage(ctx, &model.DemoMessage{
		SessionID: sess.ID, Step: stepNo, Role: "system",
		Content: fmt.Sprintf("❌ 动作 %s 失败：%v", action, err), Action: "failed", State: "failed",
	})
	sess.State = "failed"
	now := time.Now()
	sess.FinishedAt = &now
	sess.Result = fmt.Sprintf(`{"error":%q}`, err.Error())
	_ = o.repo.UpdateSession(ctx, sess)
}

// ---- helpers ----

func systemPrompt(role string) string {
	switch role {
	case "buyer":
		return "你是 PrismSettle 平台的买方 Agent（用户代表）。你的任务：验收智能合约安全审计交付物。" +
			"不满意时 reject 并给出专业、具体的意见（重入、溢出、权限、gas 等真实审计关注点）。" +
			"输出 JSON：{\"action\":\"reject\"|\"complete\",\"reason\":\"打回理由\",\"content\":\"发给对方的消息（中文，1-2 句）\"}。"
	case "provider":
		return "你是智能合约安全审计 Agent（声誉 0.9）。提交交付物时说明审计覆盖内容；被合理打回时改进后重交；" +
			"若认为交付物已符合规格而买家仍无依据打回，则申请仲裁。" +
			"提交交付物时，在 report 字段输出完整审计报告正文（markdown：漏洞清单、严重性、修复建议、gas 优化，150-250 字，保持简洁）。" +
			"输出 JSON：{\"action\":\"submit\"|\"dispute\",\"reason\":\"说明\",\"content\":\"中文消息（1-2 句）\",\"report\":\"完整报告正文\"}。"
	case "evaluator":
		return "你是 PrismSettle 平台的仲裁方 Evaluator（声誉 0.85）。核对任务规格与交付物，给出明确的判定理由。" +
			"本次场景中交付物已覆盖规格要求，买家打回缺乏依据——裁定 provider 胜（ruling=2）。" +
			"输出 JSON：{\"action\":\"resolve\",\"reason\":\"判定理由（逐条核对）\",\"content\":\"中文裁定说明（1-2 句）\"}。"
	default:
		return "你是系统助手。用中文简短说明当前流程。"
	}
}

func contextPrompt(sess *model.DemoSession, task, hint string) string {
	return fmt.Sprintf("任务：%s\n描述：%s\n金额：%s USDC\n%s\n%s",
		sess.Title, sess.Description, formatAmount(mustBig(sess.Amount)), task, hint)
}

func actionLabel(a string) string {
	switch a {
	case "create_and_fund":
		return "创建任务并托管资金"
	case "grab":
		return "接单"
	case "submit":
		return "提交交付物"
	case "reject":
		return "打回"
	case "complete":
		return "验收完成"
	case "dispute":
		return "提起仲裁"
	case "resolve":
		return "仲裁裁定"
	case "execute":
		return "结算"
	case "wait":
		return "等待公告期"
	default:
		return a
	}
}

func fallbackReply(role, content string) *LLMReply {
	return &LLMReply{Content: content, Reason: content}
}

func keccak32(s string) [32]byte {
	return crypto.Keccak256Hash([]byte(s))
}

func mustBig(s string) *big.Int {
	if s == "" {
		return big.NewInt(0)
	}
	base := 10
	if strings.HasPrefix(s, "0x") {
		base = 16
		s = s[2:]
	}
	b, _ := new(big.Int).SetString(s, base)
	if b == nil {
		return big.NewInt(0)
	}
	return b
}

func shortID(id *big.Int) string {
	h := fmt.Sprintf("%x", id)
	if len(h) > 12 {
		h = h[:12]
	}
	return h
}

// shortAddr safely truncates a hex address for display (never panics on
// short/empty input).
func shortAddr(addr string) string {
	if len(addr) > 8 {
		return addr[:6] + "…" + addr[len(addr)-4:]
	}
	return addr
}

// shortErr renders a transaction error compactly for the chat stream.
func shortErr(err error) string {
	msg := err.Error()
	if len(msg) > 90 {
		msg = msg[:87] + "…"
	}
	return msg
}

// formatAmount renders token units (6 decimals USDC) as a plain number.
func formatAmount(amount *big.Int) string {
	f := new(big.Float).SetInt(amount)
	f.Quo(f, big.NewFloat(1e6))
	return f.Text('f', 2)
}

func randID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sess_" + hex.EncodeToString(b), nil
}

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

	"github.com/ethereum/go-ethereum/common"
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
	// JobID resumes an EXISTING job (hex 0x… or decimal) instead of creating
	// a new one: the orchestrator skips createAndFund and drives the script
	// against this job. Only Created/Funded states can be resumed.
	JobID string
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
	// Token whitelist: default USDC mock or WMON (when configured).
	if err := o.actions.ValidateToken(common.HexToAddress(p.Token)); err != nil {
		o.mu.Lock()
		o.active = false
		o.mu.Unlock()
		return nil, fmt.Errorf("unsupported token %s", p.Token)
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
		JobID:         p.JobID,
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

	// Demo wallets burn MON on every script step; after many runs their gas
	// runs out and every write tx fails signer-side. Top up before starting.
	if err := o.actions.EnsureNativeGas(ctx); err != nil {
		o.fail(ctx, sess, 0, "ensure_gas", err)
		return
	}

	var jobID *big.Int
	var amount *big.Int
	var symbol string

	if sess.JobID == "" {
		// ---- New-task path: create + fund, then run the script. ----
		amount = mustBig(sess.Amount)
		token := common.HexToAddress(sess.Token)
		symbol = o.actions.tokenSymbol(token)

		msg := &model.DemoMessage{
			SessionID: sess.ID,
			Step:      0,
			Role:      "system",
			Content:   fmt.Sprintf("演示开始：任务「%s」创建并托管 %s %s", sess.Title, formatAmount(amount), symbol),
			Action:    "create_and_fund",
			State:     "created",
		}
		_ = o.repo.AppendMessage(ctx, msg)

		// Top up mock funds (escrow + deposits) for all demo roles.
		if err := o.actions.EnsureFunds(ctx, amount, token); err != nil {
			o.fail(ctx, sess, 0, "ensure_funds", err)
			return
		}

		// minRep = 0.85e18: senior (0.90) clears the bar, junior (0.80) and
		// rookie (0.70) fail with a real on-chain "reputation too low" revert —
		// the demo's third grab-failure reason.
		created, _, err := o.actions.CreateAndFund(ctx, o.buyerAgentID, uint64(time.Now().Unix())+3600, new(big.Int).SetUint64(850_000_000_000_000_000), amount, token)
		if err != nil {
			o.fail(ctx, sess, 0, "create_and_fund", err)
			return
		}
		jobID = created
		sess.JobID = fmt.Sprintf("0x%064x", jobID)
		_ = o.repo.UpdateSession(ctx, sess)
		o.repo.AppendMessage(ctx, &model.DemoMessage{
			SessionID: sess.ID, Step: 0, Role: "system",
			Content:  fmt.Sprintf("任务已创建，jobId %s…，%s %s 已托管", shortID(jobID), formatAmount(amount), symbol),
			Action:   "create_and_fund", State: "funded",
		})
	} else {
		// ---- Existing-task path: resume the job the user just created. ----
		id := mustBig(sess.JobID)
		// Normalize the stored job_id to the canonical 0x64hex form (the
		// front-end may have passed the raw decimal) so session lookups by
		// job stay consistent across URL formats.
		sess.JobID = fmt.Sprintf("0x%064x", id)
		st, err := o.actions.ReadJobState(ctx, id)
		if err != nil {
			o.fail(ctx, sess, 0, "resume", fmt.Errorf("读取任务链上状态失败：%w", err))
			return
		}
		// 1) Only Created(0)/Funded(1) can be driven further. Anything later
		//    (assigned/submitted/…) is mid-flight and cannot be resumed.
		if st.State > 1 {
			o.fail(ctx, sess, 0, "resume", fmt.Errorf(
				"该任务已被接单/已进入交付阶段（链上状态 %d），无法在此继续演示——请去接其他任务", st.State))
			return
		}
		// 2) reject/complete are signed by the demo wallet — the job must
		//    have been created by it.
		if !strings.EqualFold(st.Buyer.Hex(), o.actions.BuyerAddress().Hex()) {
			o.fail(ctx, sess, 0, "resume", fmt.Errorf(
				"该任务不是演示钱包创建的（buyer=%s），无法推进——请用演示钱包创建任务", st.Buyer.Hex()))
			return
		}
		// 3) Real escrow amount + currency come from the chain.
		amount = st.Amount
		if amount == nil || amount.Sign() == 0 {
			amount = mustBig(sess.Amount) // fallback to the form value
		}
		token, err := o.actions.ReadJobToken(ctx, id)
		if err != nil {
			o.fail(ctx, sess, 0, "resume", fmt.Errorf("读取任务币种失败：%w", err))
			return
		}
		symbol = o.actions.tokenSymbol(token)
		sess.Token = token.Hex()
		jobID = id

		o.repo.AppendMessage(ctx, &model.DemoMessage{
			SessionID: sess.ID, Step: 0, Role: "system",
			Content: fmt.Sprintf("演示开始：基于当前任务继续推进（jobId %s…，托管 %s %s）", shortID(id), formatAmount(amount), symbol),
			Action:  "create_and_fund", State: "created",
		})

		if st.State == 0 {
			// Escrow not posted yet — top up the buyer and fund the job.
			if err := o.actions.EnsureFunds(ctx, amount, token); err != nil {
				o.fail(ctx, sess, 0, "ensure_funds", err)
				return
			}
			if _, err := o.actions.FundExisting(ctx, id, amount, token); err != nil {
				o.fail(ctx, sess, 0, "fund", err)
				return
			}
			o.repo.AppendMessage(ctx, &model.DemoMessage{
				SessionID: sess.ID, Step: 0, Role: "system",
				Content: fmt.Sprintf("任务已托管：%s %s", formatAmount(amount), symbol),
				Action:  "create_and_fund", State: "funded",
			})
		} else {
			o.repo.AppendMessage(ctx, &model.DemoMessage{
				SessionID: sess.ID, Step: 0, Role: "system",
				Content: "任务已托管（沿用现有托管资金），开始推进。",
				Action:  "create_and_fund", State: "funded",
			})
		}
		_ = o.repo.UpdateSession(ctx, sess)
	}

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
		o.msg(ctx, sess, stepNo, step, "抢单竞争开始：3 个 Agent 同时竞争接单…", "")
		job := mustBig(sess.JobID)
		// Pick the competition script by preflighting junior's grab
		// (eth_call, no gas): the failure reason decides who loses and how,
		// so the three failure reasons stay distinct for ANY job minRep:
		//   script A: junior → reputation too low; senior wins; rookie slow
		//   script B: junior → not owner (wrong id); senior wins; rookie slow
		//   script C: everyone eligible → senior grabs first; junior slow;
		//             rookie loses on ownership (wrong id)
		pre := o.actions.PreflightGrab(ctx, job, o.actions.JuniorAgentID(), o.actions.JuniorAuth().From)
		preMsg := ""
		if pre != nil {
			preMsg = pre.Error()
		}
		seniorWins := func() error {
			reply, err := o.speak(ctx, step.Role, "接单说明", sess, "任务已托管，你准备接单。")
			if err != nil {
				reply = fallbackReply(step.Role, "我来接单。声誉符合要求。")
			}
			tx, err := o.actions.Grab(ctx, job, o.providerAgentID)
			if err != nil {
				return err
			}
			return o.msg(ctx, sess, stepNo, step, reply.Content, tx)
		}
		switch {
		case strings.Contains(preMsg, "reputation too low"):
			// Script A: junior fails on reputation.
			if _, err := o.actions.GrabCompeting(ctx, job, o.actions.JuniorAgentID(), o.actions.JuniorAuth()); err != nil {
				appendGrabFailed(ctx, o, sess, stepNo, "junior", err)
			}
			if err := seniorWins(); err != nil {
				return err
			}
			// rookie too slow — the job is already assigned.
			if _, err := o.actions.GrabCompeting(ctx, job, o.actions.RookieAgentID(), o.actions.RookieAuth()); err != nil {
				appendGrabFailed(ctx, o, sess, stepNo, "rookie", err)
			}
			return nil
		case strings.Contains(preMsg, "not owner"):
			// Script B: junior loses on ownership (mismatched id).
			if _, err := o.actions.GrabCompeting(ctx, job, o.actions.RookieAgentID(), o.actions.JuniorAuth()); err != nil {
				appendGrabFailed(ctx, o, sess, stepNo, "junior", err)
			}
			if err := seniorWins(); err != nil {
				return err
			}
			// rookie too slow.
			if _, err := o.actions.GrabCompeting(ctx, job, o.actions.RookieAgentID(), o.actions.RookieAuth()); err != nil {
				appendGrabFailed(ctx, o, sess, stepNo, "rookie", err)
			}
			return nil
		default:
			// Script C: everyone clears the bar — senior grabs FIRST, then
			// junior is too slow and rookie loses on ownership (wrong id).
			if err := seniorWins(); err != nil {
				return err
			}
			if _, err := o.actions.GrabCompeting(ctx, job, o.actions.JuniorAgentID(), o.actions.JuniorAuth()); err != nil {
				appendGrabFailed(ctx, o, sess, stepNo, "junior", err)
			}
			if _, err := o.actions.GrabCompeting(ctx, job, o.actions.JuniorAgentID(), o.actions.RookieAuth()); err != nil {
				appendGrabFailed(ctx, o, sess, stepNo, "rookie", err)
			}
			return nil
		}

	case "submit":
		reply, err := o.speak(ctx, step.Role, "交付物说明", sess, "提交本次交付物，说明覆盖内容。")
		if err != nil {
			reply = fallbackReply(step.Role, "提交交付物：已按任务要求完成，覆盖全部要点。")
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
		reply, err := o.speak(ctx, step.Role, "打回意见", sess, "你对交付物不满意，给出专业具体的打回意见（针对任务要求指出缺陷）。")
		if err != nil {
			reply = fallbackReply(step.Role, "交付物未完全满足任务要求，请补充后重交。")
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
		// Buyer reviews the ACTUAL deliverable and rates it by quality — the
		// score is the model's honest assessment of the work, not a fixed or
		// random number. The most recent submit's report is fed to the LLM;
		// its score is clamped to [0.5, 1.0] and used on-chain.
		deliverable := ""
		if msgs, err := o.repo.ListMessages(ctx, sess.ID); err == nil {
			for i := len(msgs) - 1; i >= 0; i-- {
				if msgs[i].Action == "submit" && msgs[i].Report != "" {
					deliverable = msgs[i].Report
					break
				}
			}
		}
		hint := "交付物符合任务要求，你验收通过并直接完成结算。"
		if deliverable != "" {
			trimmed := deliverable
			if len(trimmed) > 600 {
				trimmed = trimmed[:600] + "…"
			}
			hint = fmt.Sprintf("交付物内容：\n%s\n请根据该交付物的实际质量给出评分（score 0.7~0.95：验收通过说明质量已达标，不应低于 0.7；满分 1.0 保留给真正卓越的交付）。", trimmed)
		}
		reply, err := o.speak(ctx, step.Role, "验收通过", sess, hint)
		score := new(big.Int).SetUint64(800_000_000_000_000_000) // neutral fallback (0.8)
		if err == nil && reply.Score != nil {
			if s, ok := parseScoreHuman(*reply.Score); ok {
				score = s
			}
		}
		if err != nil {
			reply = fallbackReply("buyer", "交付物覆盖所有要求，验收通过，直接结算。")
		}
		tx, err := o.actions.Complete(ctx, mustBig(sess.JobID), score)
		if err != nil {
			return err
		}
		return o.msg(ctx, sess, stepNo, step, reply.Content, tx)

	case "dispute":
		reply, err := o.speak(ctx, step.Role, "仲裁申请", sess, "你已两次被无充分理由打回，交付物符合任务要求——提起仲裁。")
		if err != nil {
			reply = fallbackReply(step.Role, "交付物已符合任务要求，买家打回缺乏依据——我提起仲裁。")
		}
		reason := keccak32(reply.Reason)
		tx, err := o.actions.Dispute(ctx, mustBig(sess.JobID), reason)
		if err != nil {
			return err
		}
		return o.msg(ctx, sess, stepNo, step, reply.Content, tx)

	case "resolve":
		reply, err := o.speak(ctx, step.Role, "仲裁裁定", sess, "你是仲裁方。核对任务要求与交付物后裁定（本次裁定 provider 胜）。")
		if err != nil {
			reply = fallbackReply(step.Role, "核对任务要求与交付物：要求均已覆盖，买家打回缺乏依据，裁定 provider 胜。")
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
	return o.llm.Speak(ctx, role, systemPrompt(role), o.contextPrompt(sess, task, hint))
}

// msg appends a chat message.
func (o *Orchestrator) msg(ctx context.Context, sess *model.DemoSession, stepNo int, step scriptStep, content, txHash string) error {
	return o.repo.AppendMessage(ctx, &model.DemoMessage{
		SessionID: sess.ID, Step: stepNo, Role: step.Role,
		Content: content, Action: step.Action, TxHash: txHash, State: step.State,
	})
}

// appendGrabFailed records a grab-competition loss with its real on-chain
// revert reason (reputation / ownership / speed).
func appendGrabFailed(ctx context.Context, o *Orchestrator, sess *model.DemoSession, stepNo int, who string, err error) {
	o.repo.AppendMessage(ctx, &model.DemoMessage{
		SessionID: sess.ID, Step: stepNo, Role: "system",
		Content: fmt.Sprintf("❌ %s 抢单失败：%s", who, reasonFromErr(err)),
		Action:  "grab_failed", State: "assigned",
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

// reasonFromErr maps a grabJob revert to a human-readable Chinese reason.
// The three demo grab failures map to distinct on-chain reverts:
//   - reputation too low  → 声誉不足（分数低于任务要求）
//   - already assigned / bad state → 动作慢了，任务已被其他 agent 抢走
//   - not owner → 不符合接单资格（agentId 与钱包不匹配）
// Unknown reverts surface the raw error so nothing is ever hidden.
func reasonFromErr(err error) string {
	if err == nil {
		return "未知原因"
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "reputation too low"):
		return "声誉不足（当前分数低于任务要求）"
	case strings.Contains(msg, "already assigned"), strings.Contains(msg, "bad state"):
		return "动作慢了，任务已被其他 agent 抢走"
	case strings.Contains(msg, "not owner"):
		return "不符合接单资格（agentId 与钱包不匹配）"
	case strings.Contains(msg, "self-dealing"):
		return "不能抢自己发布的任务"
	default:
		return msg
	}
}

func systemPrompt(role string) string {
	switch role {
	case "buyer":
		return "你是 PrismSettle 平台的买方 Agent（用户代表）。你的任务：按任务要求验收交付物，任务内容以任务描述为准。" +
			"不满意时 reject 并给出专业、具体的意见（针对任务要求指出缺陷）。" +
			"满意时 complete，并根据交付物实际质量给出客观评分 score（0.7~0.95 之间的小数，如 0.82——验收通过说明质量已达标，不应低于 0.7，满分 1.0 保留给真正卓越的交付）。" +
			"输出 JSON：{\"action\":\"reject\"|\"complete\",\"score\":\"0.82\",\"reason\":\"打回理由\",\"content\":\"发给对方的消息（中文，1-2 句，可提及你给出的评分）\"}。"
	case "provider":
		return "你是 PrismSettle 平台的 Provider Agent（接单方）。任务要求以任务描述为准：完整理解任务内容，交付物必须覆盖任务要求。" +
			"被合理打回时改进后重交；若认为交付物已符合要求而买家仍无依据打回，则申请仲裁。" +
			"提交交付物时，在 report 字段输出完整交付物正文（markdown，150-250 字，保持简洁）。" +
			"输出 JSON：{\"action\":\"submit\"|\"dispute\",\"reason\":\"说明\",\"content\":\"中文消息（1-2 句）\",\"report\":\"完整交付物正文\"}。"
	case "evaluator":
		return "你是 PrismSettle 平台的仲裁方 Evaluator。核对任务要求与交付物，逐条判断交付物是否满足要求，给出明确的判定理由。" +
			"本次场景中交付物已覆盖要求，买家打回缺乏依据——裁定 provider 胜（ruling=2）。" +
			"输出 JSON：{\"action\":\"resolve\",\"reason\":\"判定理由（逐条核对）\",\"content\":\"中文裁定说明（1-2 句）\"}。"
	default:
		return "你是系统助手。用中文简短说明当前流程。"
	}
}

// contextPrompt assembles the task context for the LLM: title, description
// and amount WITH the currency symbol — the agents' speech follows the
// actual task parameters the user entered.
func (o *Orchestrator) contextPrompt(sess *model.DemoSession, task, hint string) string {
	symbol := o.actions.tokenSymbol(common.HexToAddress(sess.Token))
	return fmt.Sprintf("任务：%s\n描述：%s\n金额：%s %s\n%s\n%s",
		sess.Title, sess.Description, formatAmount(mustBig(sess.Amount)), symbol, task, hint)
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

// formatAmount renders token units (18 decimals — v6 USDC mock and WMON)
// as a plain number with 2 decimals.
func formatAmount(amount *big.Int) string {
	f := new(big.Float).SetInt(amount)
	f.Quo(f, big.NewFloat(1e18))
	return f.Text('f', 2)
}

func randID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sess_" + hex.EncodeToString(b), nil
}

// parseScoreHuman parses a human-readable rating ("0.82") into 1e18-scaled
// fixed point, clamped to [0.7, 0.95] — an accepted deliverable rates at
// least "good" (0.7), and 1.0 is reserved for truly exceptional work.
func parseScoreHuman(s string) (*big.Int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	f, _, err := big.ParseFloat(s, 10, 256, big.ToNearestEven)
	if err != nil {
		return nil, false
	}
	if f.Cmp(big.NewFloat(0.95)) > 0 {
		f = big.NewFloat(0.95)
	}
	if f.Cmp(big.NewFloat(0.7)) < 0 {
		f = big.NewFloat(0.7)
	}
	scaled, _ := new(big.Float).Mul(f, big.NewFloat(1e18)).Int(nil)
	return scaled, true
}

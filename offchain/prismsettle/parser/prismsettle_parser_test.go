package parser

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/zane/web3-offchain/model"
)

// makeValidationLog builds a ValidationSubmitted log identical in layout to
// what PrismSettleRegistry emits:
//
//	topics: [sig, agentId, shard, validator]
//	data:   score(32) | proofHash(32) | timestamp(32) | source(32) | jobId(32) = 160 bytes
func makeValidationLog(agentID *big.Int, shard uint8, validator common.Address, score *big.Int, proof common.Hash, ts uint64, source uint8, jobID *big.Int) types.Log {
	data := make([]byte, 160)
	score.FillBytes(data[0:32])
	copy(data[32:64], proof.Bytes())
	new(big.Int).SetUint64(ts).FillBytes(data[64:96])
	// source is uint8 — last byte of slot 3
	data[96+31] = source
	jobID.FillBytes(data[128:160])

	return types.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics: []common.Hash{
			EventValidationSubmittedSig,
			common.BytesToHash(agentID.Bytes()),
			common.BytesToHash([]byte{shard}),
			common.BytesToHash(validator.Bytes()),
		},
		Data:   data,
		TxHash: common.HexToHash("0xabcd"),
	}
}

func TestPrismSettleParser_Name(t *testing.T) {
	p := &PrismSettleParser{}
	if got := p.Name(); got != "PrismSettle" {
		t.Fatalf("Name() = %q, want %q", got, "PrismSettle")
	}
}

func TestPrismSettleParser_Match(t *testing.T) {
	p := &PrismSettleParser{}
	cases := []struct {
		name string
		sig  common.Hash
		want bool
	}{
		{"AgentRegistered", EventAgentRegisteredSig, true},
		{"ValidationSubmitted", EventValidationSubmittedSig, true},
		{"Aggregated", EventAggregatedSig, true},
		{"Staked", EventStakedSig, true},
		{"Slashed", EventSlashedSig, true},
		{"UnstakeStarted", EventUnstakeStartedSig, true},
		{"UnstakeWithdrawn", EventUnstakeWithdrawnSig, true},
		{"unknown", common.HexToHash("0xdead"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			log := types.Log{Topics: []common.Hash{c.sig}}
			if got := p.Match(log); got != c.want {
				t.Fatalf("Match() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPrismSettleParser_ParseValidationSubmitted(t *testing.T) {
	p := &PrismSettleParser{}

	agentID := big.NewInt(0x1111)
	shard := uint8(0x11)
	validator := common.HexToAddress("0xA11CE")
	score := new(big.Int).SetUint64(0.5e18)
	proof := common.HexToHash("0xproof")
	ts := uint64(1_700_000_000)
	source := uint8(1) // evaluator channel
	jobID := big.NewInt(42)

	log := makeValidationLog(agentID, shard, validator, score, proof, ts, source, jobID)
	data, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	ce, ok := data.(*model.ChainEvent)
	if !ok {
		t.Fatalf("Parse() returned %T, want *model.ChainEvent", data)
	}

	if ce.EventType != model.TypePrismValidationSubmitted {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismValidationSubmitted)
	}
	if ce.From != validator.Hex() {
		t.Errorf("From = %q, want %q", ce.From, validator.Hex())
	}
	if ce.To != common.BytesToHash(agentID.Bytes()).Hex() {
		t.Errorf("To = %q, want %q", ce.To, common.BytesToHash(agentID.Bytes()).Hex())
	}
	if ce.Value != score.String() {
		t.Errorf("Value = %q, want %q", ce.Value, score.String())
	}
	// FR-JI04: proofHash stored in TokenAddr
	if ce.TokenAddr != proof.Hex() {
		t.Errorf("TokenAddr (proofHash) = %q, want %q", ce.TokenAddr, proof.Hex())
	}
	// source stored in Symbol
	if ce.Symbol != "1" {
		t.Errorf("Symbol (source) = %q, want %q", ce.Symbol, "1")
	}
	if ce.Contract != "0x1234567890123456789012345678901234567890" {
		t.Errorf("Contract = %q, want %q", ce.Contract, "0x1234567890123456789012345678901234567890")
	}
}

func TestPrismSettleParser_ParseAggregated(t *testing.T) {
	p := &PrismSettleParser{}

	agentID := big.NewInt(0x2222)
	oldScore := big.NewInt(100)
	newScore := big.NewInt(750)
	count := big.NewInt(2)
	taskCount := big.NewInt(8)
	decay := big.NewInt(5)

	// Layout: oldScore(32) | newScore(32) | count(32) | taskCount(32) | decay(32) = 160 bytes
	data := make([]byte, 160)
	oldScore.FillBytes(data[0:32])
	newScore.FillBytes(data[32:64])
	count.FillBytes(data[64:96])
	taskCount.FillBytes(data[96:128])
	decay.FillBytes(data[128:160])

	log := types.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics: []common.Hash{
			EventAggregatedSig,
			common.BytesToHash(agentID.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismAggregated {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismAggregated)
	}
	if ce.Value != newScore.String() {
		t.Errorf("Value = %q, want %q (newScore)", ce.Value, newScore.String())
	}
	if ce.To != common.BytesToHash(agentID.Bytes()).Hex() {
		t.Errorf("To = %q, want %q", ce.To, common.BytesToHash(agentID.Bytes()).Hex())
	}
}

func TestPrismSettleParser_ParseStaked(t *testing.T) {
	p := &PrismSettleParser{}

	validator := common.HexToAddress("0xB0B")
	amount := big.NewInt(1_000_000)

	data := make([]byte, 32)
	amount.FillBytes(data)

	log := types.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics: []common.Hash{
			EventStakedSig,
			common.BytesToHash(validator.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismStaked {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismStaked)
	}
	if ce.From != validator.Hex() {
		t.Errorf("From = %q, want %q", ce.From, validator.Hex())
	}
	if ce.Value != amount.String() {
		t.Errorf("Value = %q, want %q", ce.Value, amount.String())
	}
}

func TestPrismSettleParser_ParseSlashed(t *testing.T) {
	p := &PrismSettleParser{}

	validator := common.HexToAddress("0xCAFE")
	amount := big.NewInt(500)
	reason := common.HexToHash("0xreason")

	data := make([]byte, 64)
	amount.FillBytes(data[0:32])
	copy(data[32:64], reason.Bytes())

	log := types.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics: []common.Hash{
			EventSlashedSig,
			common.BytesToHash(validator.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismSlashed {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismSlashed)
	}
	if ce.From != validator.Hex() {
		t.Errorf("From = %q, want %q", ce.From, validator.Hex())
	}
	if ce.Value != amount.String() {
		t.Errorf("Value = %q, want %q", ce.Value, amount.String())
	}
	// evidenceHash stored in TokenAddr
	if ce.TokenAddr != reason.Hex() {
		t.Errorf("TokenAddr (evidenceHash) = %q, want %q", ce.TokenAddr, reason.Hex())
	}
}

func TestPrismSettleParser_ParseUnstakeStarted(t *testing.T) {
	p := &PrismSettleParser{}

	validator := common.HexToAddress("0xD00D")
	amount := big.NewInt(2_000)
	unlockAt := uint64(1_800_000_000)

	data := make([]byte, 64)
	amount.FillBytes(data[0:32])
	new(big.Int).SetUint64(unlockAt).FillBytes(data[32:64])

	log := types.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics: []common.Hash{
			EventUnstakeStartedSig,
			common.BytesToHash(validator.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismUnstakeStarted {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismUnstakeStarted)
	}
	if ce.From != validator.Hex() {
		t.Errorf("From = %q, want %q", ce.From, validator.Hex())
	}
	if ce.Value != amount.String() {
		t.Errorf("Value = %q, want %q", ce.Value, amount.String())
	}
}

func TestPrismSettleParser_ParseUnstakeWithdrawn(t *testing.T) {
	p := &PrismSettleParser{}

	validator := common.HexToAddress("0xFACE")
	amount := big.NewInt(3_000)

	data := make([]byte, 32)
	amount.FillBytes(data)

	log := types.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics: []common.Hash{
			EventUnstakeWithdrawnSig,
			common.BytesToHash(validator.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismUnstakeWithdrawn {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismUnstakeWithdrawn)
	}
	if ce.From != validator.Hex() {
		t.Errorf("From = %q, want %q", ce.From, validator.Hex())
	}
	if ce.Value != amount.String() {
		t.Errorf("Value = %q, want %q", ce.Value, amount.String())
	}
}

func TestPrismSettleParser_ParseAgentRegistered(t *testing.T) {
	p := &PrismSettleParser{}

	agentID := big.NewInt(0x3333)
	owner := common.HexToAddress("0x0FF1CE")

	// AgentRegistered has dynamic string metadata, but parser doesn't decode it.
	// Pass empty data — parser only reads indexed topics.
	log := types.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics: []common.Hash{
			EventAgentRegisteredSig,
			common.BytesToHash(agentID.Bytes()),
			common.BytesToHash(owner.Bytes()),
		},
		Data: []byte{},
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismAgentRegistered {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismAgentRegistered)
	}
	if ce.From != owner.Hex() {
		t.Errorf("From = %q, want %q", ce.From, owner.Hex())
	}
	if ce.To != common.BytesToHash(agentID.Bytes()).Hex() {
		t.Errorf("To = %q, want %q", ce.To, common.BytesToHash(agentID.Bytes()).Hex())
	}
}

func TestPrismSettleParser_RejectsMalformed(t *testing.T) {
	p := &PrismSettleParser{}

	cases := []struct {
		name string
		log  types.Log
	}{
		{"no topics", types.Log{}},
		{"validation missing topic", types.Log{
			Topics: []common.Hash{EventValidationSubmittedSig},
			Data:   make([]byte, 160),
		}},
		{"validation data too short", types.Log{
			Topics: []common.Hash{
				EventValidationSubmittedSig,
				common.Hash{},
				common.Hash{},
				common.Hash{},
			},
			Data: make([]byte, 32),
		}},
		{"aggregated data too short", types.Log{
			Topics: []common.Hash{
				EventAggregatedSig,
				common.Hash{},
			},
			Data: make([]byte, 96), // need 160
		}},
		{"unknown sig", types.Log{
			Topics: []common.Hash{common.HexToHash("0xdead")},
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := p.Parse(c.log); err == nil {
				t.Fatal("Parse() expected error, got nil")
			}
		})
	}
}

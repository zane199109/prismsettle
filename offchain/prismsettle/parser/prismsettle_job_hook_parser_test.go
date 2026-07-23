package parser

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/zane/web3-offchain/model"
)

// Job/Hook parser tests cover all 8 events (6 Job + 2 Hook) plus malformed
// inputs. Follows the same pattern as prismsettle_parser_test.go.

func TestPrismSettleJobParser_Name(t *testing.T) {
	p := &PrismSettleJobParser{}
	if got := p.Name(); got != "PrismSettleJob" {
		t.Fatalf("Name() = %q, want %q", got, "PrismSettleJob")
	}
}

func TestPrismSettleJobParser_Match(t *testing.T) {
	p := &PrismSettleJobParser{}
	cases := []struct {
		name string
		sig  common.Hash
		want bool
	}{
		{"JobCreated", EventJobCreatedSig, true},
		{"Funded", EventFundedSig, true},
		{"Assigned", EventAssignedSig, true},
		{"Submitted", EventSubmittedSig, true},
		{"Completed", EventCompletedSig, true},
		{"Refunded", EventRefundedSig, true},
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

func TestPrismSettleJobParser_ParseJobCreated(t *testing.T) {
	p := &PrismSettleJobParser{}

	agentID := big.NewInt(0x1111)
	jobID := big.NewInt(99)
	buyer := common.HexToAddress("0xBA5E")
	deadline := uint64(2_000_000_000)
	hook := common.HexToAddress("0xH00K")

	// Layout: buyer(32) | deadline(32) | hook(32) = 96 bytes
	// Solidity ABI right-aligns addresses in 32-byte slots.
	data := make([]byte, 96)
	copy(data[12:32], buyer.Bytes())
	new(big.Int).SetUint64(deadline).FillBytes(data[32:64])
	copy(data[76:96], hook.Bytes())

	log := types.Log{
		Address: common.HexToAddress("0xJobContract"),
		Topics: []common.Hash{
			EventJobCreatedSig,
			common.BytesToHash(agentID.Bytes()),
			common.BytesToHash(jobID.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismJobCreated {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismJobCreated)
	}
	// From = agentId
	if ce.From != common.BytesToHash(agentID.Bytes()).Hex() {
		t.Errorf("From (agentId) = %q, want %q", ce.From, common.BytesToHash(agentID.Bytes()).Hex())
	}
	// To = jobId
	if ce.To != common.BytesToHash(jobID.Bytes()).Hex() {
		t.Errorf("To (jobId) = %q, want %q", ce.To, common.BytesToHash(jobID.Bytes()).Hex())
	}
	// FR-JI05: hook address stored in TokenAddr
	if ce.TokenAddr != hook.Hex() {
		t.Errorf("TokenAddr (hook) = %q, want %q", ce.TokenAddr, hook.Hex())
	}
	// buyer stored in Symbol
	if ce.Symbol != buyer.Hex() {
		t.Errorf("Symbol (buyer) = %q, want %q", ce.Symbol, buyer.Hex())
	}
}

func TestPrismSettleJobParser_ParseFunded(t *testing.T) {
	p := &PrismSettleJobParser{}

	jobID := big.NewInt(100)
	buyer := common.HexToAddress("0xBA5E")
	amount := big.NewInt(1_000_000_000_000_000_000) // 1 ether

	// Layout: buyer(32) | amount(32) = 64 bytes
	// Solidity ABI right-aligns addresses in 32-byte slots.
	data := make([]byte, 64)
	copy(data[12:32], buyer.Bytes())
	amount.FillBytes(data[32:64])

	log := types.Log{
		Address: common.HexToAddress("0xJobContract"),
		Topics: []common.Hash{
			EventFundedSig,
			common.BytesToHash(jobID.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismJobFunded {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismJobFunded)
	}
	if ce.To != common.BytesToHash(jobID.Bytes()).Hex() {
		t.Errorf("To (jobId) = %q, want %q", ce.To, common.BytesToHash(jobID.Bytes()).Hex())
	}
	if ce.From != buyer.Hex() {
		t.Errorf("From (buyer) = %q, want %q", ce.From, buyer.Hex())
	}
	if ce.Value != amount.String() {
		t.Errorf("Value = %q, want %q", ce.Value, amount.String())
	}
}

func TestPrismSettleJobParser_ParseAssigned(t *testing.T) {
	p := &PrismSettleJobParser{}

	jobID := big.NewInt(101)
	provider := common.HexToAddress("0xPR0V")

	// Solidity ABI right-aligns addresses in 32-byte slots.
	data := make([]byte, 32)
	copy(data[12:32], provider.Bytes())

	log := types.Log{
		Address: common.HexToAddress("0xJobContract"),
		Topics: []common.Hash{
			EventAssignedSig,
			common.BytesToHash(jobID.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismJobAssigned {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismJobAssigned)
	}
	if ce.To != common.BytesToHash(jobID.Bytes()).Hex() {
		t.Errorf("To (jobId) = %q, want %q", ce.To, common.BytesToHash(jobID.Bytes()).Hex())
	}
	if ce.From != provider.Hex() {
		t.Errorf("From (provider) = %q, want %q", ce.From, provider.Hex())
	}
}

func TestPrismSettleJobParser_ParseSubmitted(t *testing.T) {
	p := &PrismSettleJobParser{}

	jobID := big.NewInt(102)
	deliverableHash := common.HexToHash("0xdeliv")
	proofHash := common.HexToHash("0xpro0f")

	// Layout: deliverableHash(32) | proofHash(32) = 64 bytes
	data := make([]byte, 64)
	copy(data[0:32], deliverableHash.Bytes())
	copy(data[32:64], proofHash.Bytes())

	log := types.Log{
		Address: common.HexToAddress("0xJobContract"),
		Topics: []common.Hash{
			EventSubmittedSig,
			common.BytesToHash(jobID.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismJobSubmitted {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismJobSubmitted)
	}
	if ce.To != common.BytesToHash(jobID.Bytes()).Hex() {
		t.Errorf("To (jobId) = %q, want %q", ce.To, common.BytesToHash(jobID.Bytes()).Hex())
	}
	// FR-JI04: deliverableHash in TokenAddr, proofHash in From
	if ce.TokenAddr != deliverableHash.Hex() {
		t.Errorf("TokenAddr (deliverableHash) = %q, want %q", ce.TokenAddr, deliverableHash.Hex())
	}
	if ce.From != proofHash.Hex() {
		t.Errorf("From (proofHash) = %q, want %q", ce.From, proofHash.Hex())
	}
}

func TestPrismSettleJobParser_ParseCompleted(t *testing.T) {
	p := &PrismSettleJobParser{}

	jobID := big.NewInt(103)
	provider := common.HexToAddress("0xPR0V")
	amount := big.NewInt(2_000)

	// Solidity ABI right-aligns addresses in 32-byte slots.
	data := make([]byte, 64)
	copy(data[12:32], provider.Bytes())
	amount.FillBytes(data[32:64])

	log := types.Log{
		Address: common.HexToAddress("0xJobContract"),
		Topics: []common.Hash{
			EventCompletedSig,
			common.BytesToHash(jobID.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismJobCompleted {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismJobCompleted)
	}
	if ce.To != common.BytesToHash(jobID.Bytes()).Hex() {
		t.Errorf("To (jobId) = %q, want %q", ce.To, common.BytesToHash(jobID.Bytes()).Hex())
	}
	if ce.From != provider.Hex() {
		t.Errorf("From (provider) = %q, want %q", ce.From, provider.Hex())
	}
	if ce.Value != amount.String() {
		t.Errorf("Value = %q, want %q", ce.Value, amount.String())
	}
}

func TestPrismSettleJobParser_ParseRefunded(t *testing.T) {
	p := &PrismSettleJobParser{}

	jobID := big.NewInt(104)
	buyer := common.HexToAddress("0xBA5E")
	amount := big.NewInt(1_500)

	// Solidity ABI right-aligns addresses in 32-byte slots.
	data := make([]byte, 64)
	copy(data[12:32], buyer.Bytes())
	amount.FillBytes(data[32:64])

	log := types.Log{
		Address: common.HexToAddress("0xJobContract"),
		Topics: []common.Hash{
			EventRefundedSig,
			common.BytesToHash(jobID.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismJobRefunded {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismJobRefunded)
	}
	if ce.To != common.BytesToHash(jobID.Bytes()).Hex() {
		t.Errorf("To (jobId) = %q, want %q", ce.To, common.BytesToHash(jobID.Bytes()).Hex())
	}
	if ce.From != buyer.Hex() {
		t.Errorf("From (buyer) = %q, want %q", ce.From, buyer.Hex())
	}
	if ce.Value != amount.String() {
		t.Errorf("Value = %q, want %q", ce.Value, amount.String())
	}
}

func TestPrismSettleJobParser_RejectsMalformed(t *testing.T) {
	p := &PrismSettleJobParser{}

	cases := []struct {
		name string
		log  types.Log
	}{
		{"no topics", types.Log{}},
		{"JobCreated missing topic", types.Log{
			Topics: []common.Hash{EventJobCreatedSig},
			Data:   make([]byte, 96),
		}},
		{"JobCreated data too short", types.Log{
			Topics: []common.Hash{
				EventJobCreatedSig,
				common.Hash{},
				common.Hash{},
			},
			Data: make([]byte, 32),
		}},
		{"Submitted data too short", types.Log{
			Topics: []common.Hash{
				EventSubmittedSig,
				common.Hash{},
			},
			Data: make([]byte, 32), // need 64
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

// --- Hook Parser Tests ---

func TestPrismSettleHookParser_Name(t *testing.T) {
	p := &PrismSettleHookParser{}
	if got := p.Name(); got != "PrismSettleHook" {
		t.Fatalf("Name() = %q, want %q", got, "PrismSettleHook")
	}
}

func TestPrismSettleHookParser_Match(t *testing.T) {
	p := &PrismSettleHookParser{}
	cases := []struct {
		name string
		sig  common.Hash
		want bool
	}{
		{"Disputed", EventDisputedSig, true},
		{"DisputeResolved", EventDisputeResolvedSig, true},
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

func TestPrismSettleHookParser_ParseDisputed(t *testing.T) {
	p := &PrismSettleHookParser{}

	jobID := big.NewInt(200)
	reasonHash := common.HexToHash("0xreas0n")

	// Disputed: reasonHash is indexed (in topics, not data)
	log := types.Log{
		Address: common.HexToAddress("0xHookContract"),
		Topics: []common.Hash{
			EventDisputedSig,
			common.BytesToHash(jobID.Bytes()),
			reasonHash,
		},
		Data: []byte{},
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismDisputed {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismDisputed)
	}
	if ce.To != common.BytesToHash(jobID.Bytes()).Hex() {
		t.Errorf("To (jobId) = %q, want %q", ce.To, common.BytesToHash(jobID.Bytes()).Hex())
	}
	// FR-JI06: reasonHash stored in TokenAddr
	if ce.TokenAddr != reasonHash.Hex() {
		t.Errorf("TokenAddr (reasonHash) = %q, want %q", ce.TokenAddr, reasonHash.Hex())
	}
}

func TestPrismSettleHookParser_ParseDisputeResolved(t *testing.T) {
	p := &PrismSettleHookParser{}

	jobID := big.NewInt(201)
	ruling := uint8(2) // Provider wins

	// DisputeResolved: ruling is uint8 in data (last byte of 32-byte slot)
	data := make([]byte, 32)
	data[31] = ruling

	log := types.Log{
		Address: common.HexToAddress("0xHookContract"),
		Topics: []common.Hash{
			EventDisputeResolvedSig,
			common.BytesToHash(jobID.Bytes()),
		},
		Data: data,
	}

	out, err := p.Parse(log)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	ce := out.(*model.ChainEvent)

	if ce.EventType != model.TypePrismDisputeResolved {
		t.Errorf("EventType = %q, want %q", ce.EventType, model.TypePrismDisputeResolved)
	}
	if ce.To != common.BytesToHash(jobID.Bytes()).Hex() {
		t.Errorf("To (jobId) = %q, want %q", ce.To, common.BytesToHash(jobID.Bytes()).Hex())
	}
	if ce.Value != "2" {
		t.Errorf("Value (ruling) = %q, want %q", ce.Value, "2")
	}
}

func TestPrismSettleHookParser_RejectsMalformed(t *testing.T) {
	p := &PrismSettleHookParser{}

	cases := []struct {
		name string
		log  types.Log
	}{
		{"no topics", types.Log{}},
		{"Disputed missing topic", types.Log{
			Topics: []common.Hash{EventDisputedSig},
		}},
		{"DisputeResolved data too short", types.Log{
			Topics: []common.Hash{
				EventDisputeResolvedSig,
				common.Hash{},
			},
			Data: make([]byte, 16), // need 32
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

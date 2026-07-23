package repository

import (
	"context"
	"math/big"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/constant"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NOTE: EventIngestService.RollbackEvents guard is tested in
// internal/service/event_ingest_service_test.go (different package).

// newTestDB spins up an in-memory SQLite gorm DB with the chain_events schema,
// so the real SQL in erc20_queries.go / chain_event_repository.go is exercised
// without needing a Postgres instance.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ChainEvent{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	// wipe between tests since cache=shared keeps the in-memory db alive
	if err := db.Exec(`DELETE FROM chain_events`).Error; err != nil {
		t.Fatalf("clean chain_events: %v", err)
	}
	return db
}

// ev builds a ChainEvent with sensible defaults; callers override fields.
func ev(chain, from, to, value, contract, tokenAddr string, block uint64, txIdx string) *model.ChainEvent {
	return &model.ChainEvent{
		ChainName:   chain,
		ChainType:   "evm",
		TxType:      constant.TxTypeToken,
		EventType:   model.TypeERC20Transfer,
		TokenAddr:   tokenAddr,
		From:        from,
		To:          to,
		Value:       value,
		Symbol:      "TST",
		Decimals:    18,
		TxHash:      txIdx,
		LogIndex:    0,
		BlockNumber: block,
		BlockTime:   block * 10, // deterministic on-chain time
		Contract:    contract,
		Status:      1,
	}
}

// ---------------------------------------------------------------------------
// Balance calculation
// ---------------------------------------------------------------------------

// TestGetTokenBalance verifies balance = incoming - outgoing, isolated by
// contract address (cross-token events must not leak into the result).
func TestGetTokenBalance(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)
	ctx := context.Background()

	tokenA := "0xTokenA"
	alice, bob := "0xAlice", "0xBob"

	events := []*model.ChainEvent{
		ev("eth", bob, alice, "1000", tokenA, tokenA, 1, "0xtx1"),           // +1000 in
		ev("eth", alice, bob, "300", tokenA, tokenA, 2, "0xtx2"),            //  -300 out
		ev("eth", bob, alice, "999999", "0xTokenB", "0xTokenB", 3, "0xtx3"), // different token: must be ignored
	}
	for _, e := range events {
		e.LogIndex = uint64(0)
	}
	// disambiguate log indexes
	events[1].LogIndex = 1
	events[2].LogIndex = 2
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	bal, err := repo.GetTokenBalance(ctx, "eth", alice, tokenA)
	if err != nil {
		t.Fatalf("GetTokenBalance: %v", err)
	}
	if bal.Cmp(big.NewInt(700)) != 0 {
		t.Fatalf("balance = %s, want 700", bal.String())
	}
}

// TestGetTokenBalance_CaseInsensitiveContract is the regression for the cache-key
// class of bug: contract is stored mixed-case, query must still match via LOWER().
func TestGetTokenBalance_CaseInsensitiveContract(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)
	ctx := context.Background()

	// stored as EIP-55 mixed case
	mixedCase := "0x5FbDB2315678afecb367f032d93F642f64180aa3"
	events := []*model.ChainEvent{
		ev("eth", "0xSender", "0xAlice", "500", mixedCase, mixedCase, 1, "0xtx1"),
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	// query with a different case spelling
	bal, err := repo.GetTokenBalance(ctx, "eth", "0xAlice", "0x5fbdb2315678afecb367f032d93f642f64180aa3")
	if err != nil {
		t.Fatalf("GetTokenBalance: %v", err)
	}
	if bal.Cmp(big.NewInt(500)) != 0 {
		t.Fatalf("balance = %s, want 500 (case-insensitive match)", bal.String())
	}
}

// TestGetTokenBalance_NoTransfers verifies an address with no events returns 0,
// not an error (COALESCE contract).
func TestGetTokenBalance_NoTransfers(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)

	bal, err := repo.GetTokenBalance(context.Background(), "eth", "0xNobody", "0xTokenA")
	if err != nil {
		t.Fatalf("GetTokenBalance on empty: %v", err)
	}
	if bal.Sign() != 0 {
		t.Fatalf("balance = %s, want 0", bal.String())
	}
}

// TestGetNativeBalance_ZeroAddress is one half of the P1 regression: native
// events written with token_addr = 0x0000...0000 must be counted.
func TestGetNativeBalance_ZeroAddress(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)
	ctx := context.Background()

	alice := "0xAlice"
	events := []*model.ChainEvent{
		// native funded: token_addr = zero address
		ev("eth", "0xFunder", alice, "1000", "0xFundMe", constant.NativeTokenAddress, 1, "0xtx1"),
		ev("eth", "0xFunder", alice, "500", "0xFundMe", constant.NativeTokenAddress, 2, "0xtx2"),
		ev("eth", alice, "0xWithdraw", "300", "0xFundMe", constant.NativeTokenAddress, 3, "0xtx3"),
	}
	events[1].LogIndex = 1
	events[2].LogIndex = 2
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	bal, err := repo.GetNativeBalance(ctx, "eth", alice)
	if err != nil {
		t.Fatalf("GetNativeBalance: %v", err)
	}
	// 1000 + 500 - 300 = 1200
	if bal.Cmp(big.NewInt(1200)) != 0 {
		t.Fatalf("native balance = %s, want 1200", bal.String())
	}
}

// TestGetNativeBalance_ETHPlaceholder is the other half of the P1 regression:
// native events written with token_addr = 0xEeee...EeeeE (DeFi placeholder used
// by fundme_parser) must also be counted. Before the P1 fix, the query filtered
// on contract=” / '0x0000...' and NEVER matched these events, so the native
// balance was silently 0.
func TestGetNativeBalance_ETHPlaceholder(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)
	ctx := context.Background()

	alice := "0xAlice"
	events := []*model.ChainEvent{
		ev("eth", "0xFunder", alice, "2000", "0xFundMe", constant.NativeETHPlaceholder, 1, "0xtx1"),
		ev("eth", alice, "0xOut", "500", "0xFundMe", constant.NativeETHPlaceholder, 2, "0xtx2"),
	}
	events[1].LogIndex = 1
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	bal, err := repo.GetNativeBalance(ctx, "eth", alice)
	if err != nil {
		t.Fatalf("GetNativeBalance: %v", err)
	}
	if bal.Cmp(big.NewInt(1500)) != 0 {
		t.Fatalf("native balance = %s, want 1500", bal.String())
	}
}

// TestGetNativeBalance_MixedNativeRepresentations verifies both native
// representations in the same ledger are aggregated together.
func TestGetNativeBalance_MixedNativeRepresentations(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)
	ctx := context.Background()

	alice := "0xAlice"
	events := []*model.ChainEvent{
		ev("eth", "0xFunder", alice, "1000", "0xFundMe", constant.NativeTokenAddress, 1, "0xtx1"),
		ev("eth", "0xFunder", alice, "2000", "0xFundMe", constant.NativeETHPlaceholder, 2, "0xtx2"),
	}
	events[1].LogIndex = 1
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	bal, err := repo.GetNativeBalance(ctx, "eth", alice)
	if err != nil {
		t.Fatalf("GetNativeBalance: %v", err)
	}
	if bal.Cmp(big.NewInt(3000)) != 0 {
		t.Fatalf("native balance = %s, want 3000 (mixed representations)", bal.String())
	}
}

// TestGetNativeBalance_IgnoresERC20 ensures ERC20 transfers (with a real
// token_addr) are NOT counted as native balance.
func TestGetNativeBalance_IgnoresERC20(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)
	ctx := context.Background()

	alice := "0xAlice"
	erc20 := "0xRealERC20"
	events := []*model.ChainEvent{
		// ERC20 transfer to alice - must not count toward native balance
		ev("eth", "0xSender", alice, "999999", erc20, erc20, 1, "0xtx1"),
		// real native transfer
		ev("eth", "0xFunder", alice, "100", "0xFundMe", constant.NativeTokenAddress, 2, "0xtx2"),
	}
	events[1].LogIndex = 1
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	bal, err := repo.GetNativeBalance(ctx, "eth", alice)
	if err != nil {
		t.Fatalf("GetNativeBalance: %v", err)
	}
	if bal.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("native balance = %s, want 100 (ERC20 events excluded)", bal.String())
	}
}

// ---------------------------------------------------------------------------
// Reorg rollback
// ---------------------------------------------------------------------------

// TestRollbackEvents deletes events in [fromBlock, toBlock] only, leaving
// earlier and later blocks intact. This is the data layer of reorg handling.
func TestRollbackEvents(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)
	ctx := context.Background()

	contract := "0xFundMe"
	var events []*model.ChainEvent
	for b := uint64(1); b <= 20; b++ {
		events = append(events, ev("eth", "0xA", "0xB", "1", contract, constant.NativeETHPlaceholder, b, txHashFor(b)))
	}
	for i := range events {
		events[i].LogIndex = uint64(i)
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	// simulate reorg: rollback blocks 11..20
	if _, err := repo.RollbackEvents(ctx, "eth", contract, 11, 20); err != nil {
		t.Fatalf("RollbackEvents: %v", err)
	}

	var remaining []model.ChainEvent
	if err := db.Find(&remaining).Error; err != nil {
		t.Fatalf("query remaining: %v", err)
	}
	if len(remaining) != 10 {
		t.Fatalf("after rollback %d events remain, want 10 (blocks 1-10)", len(remaining))
	}
	for _, e := range remaining {
		if e.BlockNumber > 10 {
			t.Errorf("block %d should have been rolled back", e.BlockNumber)
		}
	}
}

// TestRollbackEvents_IsolatesByContract ensures rollback of one contract
// does not touch another contract's events.
func TestRollbackEvents_IsolatesByContract(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)
	ctx := context.Background()

	c1, c2 := "0xContract1", "0xContract2"
	for b := uint64(1); b <= 5; b++ {
		e1 := ev("eth", "0xA", "0xB", "1", c1, constant.NativeETHPlaceholder, b, txHashFor(b))
		e2 := ev("eth", "0xA", "0xB", "1", c2, constant.NativeETHPlaceholder, b, "0xother"+txHashFor(b))
		e1.LogIndex = 0
		e2.LogIndex = 1
		if err := db.Create(&[]*model.ChainEvent{e1, e2}).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// rollback c1 blocks 3..5 only
	if _, err := repo.RollbackEvents(ctx, "eth", c1, 3, 5); err != nil {
		t.Fatalf("RollbackEvents: %v", err)
	}

	var c1Count, c2Count int64
	db.Model(&model.ChainEvent{}).Where("contract = ?", c1).Count(&c1Count)
	db.Model(&model.ChainEvent{}).Where("contract = ?", c2).Count(&c2Count)
	if c1Count != 2 {
		t.Errorf("contract1 remaining = %d, want 2", c1Count)
	}
	if c2Count != 5 {
		t.Errorf("contract2 remaining = %d, want 5 (untouched)", c2Count)
	}
}

// TestBatchSaveChainEvents_Idempotent verifies ON CONFLICT DO NOTHING: inserting
// the same (tx_hash, log_index) twice does not duplicate rows.
func TestBatchSaveChainEvents_Idempotent(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)
	ctx := context.Background()

	events := []*model.ChainEvent{
		ev("eth", "0xA", "0xB", "100", "0xC", "0xC", 1, "0xSameTx"),
	}
	events[0].LogIndex = 1

	if err := repo.BatchSaveChainEvents(ctx, events); err != nil {
		t.Fatalf("first batch: %v", err)
	}
	// insert the exact same (tx_hash, log_index) again
	if err := repo.BatchSaveChainEvents(ctx, events); err != nil {
		t.Fatalf("second batch (dup): %v", err)
	}

	var count int64
	db.Model(&model.ChainEvent{}).Count(&count)
	if count != 1 {
		t.Fatalf("idempotent insert produced %d rows, want 1", count)
	}
}

// TestBatchSaveChainEvents_EmptyAndCancel verifies fast-path returns nil for
// empty input and a canceled context.
func TestBatchSaveChainEvents_EmptyAndCancel(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)

	if err := repo.BatchSaveChainEvents(context.Background(), nil); err != nil {
		t.Fatalf("empty input: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := repo.BatchSaveChainEvents(ctx, []*model.ChainEvent{ev("eth", "a", "b", "1", "c", "c", 1, "x")}); err == nil {
		t.Fatal("canceled context: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetTransfers time filter (P1 regression)
// ---------------------------------------------------------------------------

// TestGetTransfers_TimeFilterUsesBlockTime verifies the time range filter hits
// block_time (on-chain time), not created_at. Before the P1 fix this used
// to_timestamp(created_at) which was wrong field + wrong type cast.
func TestGetTransfers_TimeFilterUsesBlockTime(t *testing.T) {
	db := newTestDB(t)
	repo := NewChainEventRepository(db)
	ctx := context.Background()

	contract := "0xTokenA"
	// block_time = block * 10
	events := []*model.ChainEvent{
		ev("eth", "0xA", "0xB", "1", contract, contract, 1, "0xtx1"), // block_time=10
		ev("eth", "0xA", "0xB", "1", contract, contract, 2, "0xtx2"), // block_time=20
		ev("eth", "0xA", "0xB", "1", contract, contract, 3, "0xtx3"), // block_time=30
	}
	for i := range events {
		events[i].LogIndex = uint64(i)
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	// range [15, 25] should match ONLY block 2 (block_time=20)
	list, total, err := repo.GetTransfers(ctx, "eth", "0xA", contract, 15, 25, 1, 10)
	if err != nil {
		t.Fatalf("GetTransfers: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1 (only block 2 in [15,25])", total)
	}
	if len(list) != 1 || list[0].BlockNumber != 2 {
		t.Fatalf("got %v, want block 2", list)
	}
}

func txHashFor(b uint64) string {
	return "0xhash" + string(rune('a'-1+b))
}

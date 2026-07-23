package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/zane/web3-offchain/internal/repository"
	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/constant"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSvcTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:svc::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ChainEvent{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Exec(`DELETE FROM chain_events`).Error; err != nil {
		t.Fatalf("clean: %v", err)
	}
	return db
}

// TestEventIngestService_RollbackGuard verifies the service-layer reorg guard:
// when startRollbackBlock > dbLastBlock, RollbackEvents must be a no-op and must
// NOT delete any rows. This protects against accidental full-table wipes when
// the reorg start block ends up ahead of the DB head.
func TestEventIngestService_RollbackGuard(t *testing.T) {
	db := newSvcTestDB(t)
	repo := repository.NewChainEventRepository(db)
	svc := NewEventIngestService(repo)
	ctx := context.Background()

	contract := "0xFundMe"
	var events []*model.ChainEvent
	for b := uint64(1); b <= 5; b++ {
		events = append(events, &model.ChainEvent{
			ChainName: "eth", ChainType: "evm", TxType: constant.TxTypeToken,
			EventType: model.TypeFundMeFunded, TokenAddr: constant.NativeETHPlaceholder,
			From: "0xA", To: "0xB", Value: "1", Symbol: "ETH", Decimals: 18,
			TxHash: txHash(b), LogIndex: uint64(b), BlockNumber: b, BlockTime: b * 10,
			Contract: contract, Status: 1,
		})
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	// dbLastBlock=5, startRollback=7 -> guard returns nil, nothing deleted
	if _, err := svc.RollbackEvents(ctx, "eth", contract, 7, 5); err != nil {
		t.Fatalf("RollbackEvents guard: %v", err)
	}

	var count int64
	db.Model(&model.ChainEvent{}).Count(&count)
	if count != 5 {
		t.Fatalf("after no-op rollback %d rows, want 5 (guard must not delete)", count)
	}
}

// TestEventIngestService_RollbackDelegates verifies that when start <= dbLast,
// the service delegates to the repo and actually deletes the range.
func TestEventIngestService_RollbackDelegates(t *testing.T) {
	db := newSvcTestDB(t)
	repo := repository.NewChainEventRepository(db)
	svc := NewEventIngestService(repo)
	ctx := context.Background()

	contract := "0xFundMe"
	var events []*model.ChainEvent
	for b := uint64(1); b <= 5; b++ {
		events = append(events, &model.ChainEvent{
			ChainName: "eth", ChainType: "evm", TxType: constant.TxTypeToken,
			EventType: model.TypeFundMeFunded, TokenAddr: constant.NativeETHPlaceholder,
			From: "0xA", To: "0xB", Value: "1", Symbol: "ETH", Decimals: 18,
			TxHash: txHash(b), LogIndex: uint64(b), BlockNumber: b, BlockTime: b * 10,
			Contract: contract, Status: 1,
		})
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	// rollback 4..5 (dbLast=5)
	if _, err := svc.RollbackEvents(ctx, "eth", contract, 4, 5); err != nil {
		t.Fatalf("RollbackEvents: %v", err)
	}

	var count int64
	db.Model(&model.ChainEvent{}).Count(&count)
	if count != 3 {
		t.Fatalf("after rollback 4..5: %d rows, want 3 (blocks 1-3 remain)", count)
	}
}

// TestEventIngestService_BatchSaveEmpty verifies the fast path: empty input
// returns nil without touching the DB.
func TestEventIngestService_BatchSaveEmpty(t *testing.T) {
	db := newSvcTestDB(t)
	repo := repository.NewChainEventRepository(db)
	svc := NewEventIngestService(repo)

	if err := svc.BatchSaveChainEvents(context.Background(), nil); err != nil {
		t.Fatalf("empty batch: %v", err)
	}
	var count int64
	db.Model(&model.ChainEvent{}).Count(&count)
	if count != 0 {
		t.Fatalf("empty batch wrote %d rows, want 0", count)
	}
}

func txHash(b uint64) string {
	return "0xhash" + string(rune('a'-1+b))
}

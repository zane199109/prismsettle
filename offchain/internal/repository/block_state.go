package repository

import (
	"context"
	"errors"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BlockStateRepository handles data access for chain block synchronization state
// It stores and updates the last synced block number and hash per contract
type BlockStateRepository struct {
	db *gorm.DB
}

// NewBlockStateRepository creates a new BlockStateRepository instance
func NewBlockStateRepository(db *gorm.DB) *BlockStateRepository {
	return &BlockStateRepository{db: db}
}

// GetLastBlock retrieves the latest synced block number and hash for a specific chain + contract
// Returns (0, "", nil) if no record is found (fresh start)
func (r *BlockStateRepository) GetLastBlock(
	ctx context.Context,
	chainName string,
	contractAddr string,
) (uint64, string, error) {

	var state model.ChainBlockState

	// Query the latest sync state by chain and contract
	err := r.db.WithContext(ctx).
		Where("chain_name = ?", chainName).
		Where("contract_addr = ?", contractAddr).
		First(&state).Error

	// If no record exists, return initial state (0 height)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, "", nil
	}

	// Log and return other database errors
	if err != nil {
		logger.Errorf("failed to get last synced block",
			logger.String("chain", chainName),
			logger.String("contract", contractAddr),
			logger.Error(err),
		)
		return 0, "", err
	}
	return state.LastBlock, state.BlockHash, nil
}

// SetLastBlock updates the last synced block number and hash
// Uses ON CONFLICT to support upsert (create if not exists, update if exists)
func (r *BlockStateRepository) SetLastBlock(
	ctx context.Context,
	chainName string,
	contractAddr string,
	blockNumber uint64,
	blockHash string,
) error {

	state := model.ChainBlockState{
		ChainName:    chainName,
		ContractAddr: contractAddr,
		LastBlock:    blockNumber,
		BlockHash:    blockHash,
	}

	// Upsert: update on duplicate (chain_name + contract_addr)
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "chain_name"}, {Name: "contract_addr"}},
			DoUpdates: clause.AssignmentColumns([]string{"last_block", "block_hash", "updated_at"}),
		}).
		Create(&state).Error

	// Log and return error if failed
	if err != nil {
		logger.Errorf("failed to update last synced block",
			logger.String("chain", chainName),
			logger.String("contract", contractAddr),
			logger.Uint64("block_num", blockNumber),
			logger.String("block_hash", blockHash),
			logger.Error(err),
		)
		return err
	}
	return nil
}

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zane/web3-offchain/internal/repository"
	"github.com/zane/web3-offchain/internal/storage"
	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/constant"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
)

// SyncStateService manages blockchain synchronization state
// Including last synced block, token info cache, and chain state persistence
type SyncStateService struct {
	ceRepo         *repository.ChainEventRepository
	blockStateRepo *repository.BlockStateRepository
	redis          *storage.RedisStorage
}

// NewSyncStateService creates a new SyncStateService with dependency injection
func NewSyncStateService(
	ceRepo *repository.ChainEventRepository,
	blockStateRepo *repository.BlockStateRepository,
	redis *storage.RedisStorage,
) *SyncStateService {
	return &SyncStateService{
		ceRepo:         ceRepo,
		blockStateRepo: blockStateRepo,
		redis:          redis,
	}
}

// GetLastBlock retrieves the last synced block number and hash from database
// Used for chain sync resumption
func (s *SyncStateService) GetLastBlock(
	ctx context.Context,
	chainName string,
	contractAddr string,
) (uint64, string, error) {
	lastBlock, blockHash, err := s.blockStateRepo.GetLastBlock(ctx, chainName, contractAddr)
	if err != nil {
		logger.Errorf("get last block failed", logger.Error(err))
		return 0, "", errno.New(errno.ErrInternal.Code, "failed to get last block from DB", err)
	}

	return lastBlock, blockHash, nil
}

// SetLastBlock persists the latest synced block number and hash
// Ensures the sync process can resume after restart
func (s *SyncStateService) SetLastBlock(
	ctx context.Context,
	chainName string,
	contractAddr string,
	blockNumber uint64,
	blockHash string,
) error {
	err := s.blockStateRepo.SetLastBlock(ctx, chainName, contractAddr, blockNumber, blockHash)
	if err != nil {
		logger.Errorf("set last block failed", logger.Error(err))
		return errno.New(errno.ErrInternal.Code, "failed to set last block to DB", err)
	}

	return nil
}

// GetTokenInfo retrieves ERC20 token metadata (symbol, decimals)
// Uses cache-aside pattern: Redis cache -> DB fallback
func (s *SyncStateService) GetTokenInfo(
	ctx context.Context,
	chainName string,
	contractAddr string,
) (*model.TokenInfo, error) {
	// Normalize address to lowercase to keep cache key consistent across callers
	// (go-ethereum Address.Hex() returns EIP-55 mixed-case checksum, but API layer
	// uses strings.ToLower). Without normalization, writes and reads would never match.
	contractAddr = strings.ToLower(contractAddr)
	key := fmt.Sprintf("%s:%s:%s", constant.KeyTokenInfoPrefix, chainName, contractAddr)

	// 1. Get from Redis cache first
	cacheData, err := s.redis.Get(ctx, key)
	if err == nil && cacheData != "" {
		var tokenInfo model.TokenInfo
		if err := json.Unmarshal([]byte(cacheData), &tokenInfo); err == nil {
			return &tokenInfo, nil
		}
		logger.Warn("failed to unmarshal token info cache", logger.String("key", key))
	}

	// 2. Cache miss: query from chain_events table
	tokenInfo, err := s.ceRepo.GetTokenInfoFromEvent(ctx, chainName, contractAddr)
	if err != nil {
		return nil, err
	}

	// 3. Update cache with 24h expiration
	jsonBytes, _ := json.Marshal(tokenInfo)
	_ = s.redis.Set(ctx, key, string(jsonBytes), 24*time.Hour)

	return tokenInfo, nil
}

// SetTokenInfo stores token metadata into Redis with long TTL
// Used by chain listener to pre-cache token information
func (s *SyncStateService) SetTokenInfo(
	ctx context.Context,
	chainName string,
	tokenAddr string,
	info *model.TokenInfo,
) error {
	// Normalize address to lowercase to keep cache key consistent with GetTokenInfo.
	tokenAddr = strings.ToLower(tokenAddr)
	key := fmt.Sprintf("%s:%s:%s", constant.KeyTokenInfoPrefix, chainName, tokenAddr)

	data, err := json.Marshal(info)
	if err != nil {
		return errno.New(errno.ErrInternal.Code, "failed to marshal token info", err)
	}

	// Cache for 90 days
	err = s.redis.Set(ctx, key, string(data), 90*24*time.Hour)
	if err != nil {
		logger.Errorf("set token info cache failed", logger.Error(err))
		return errno.New(errno.ErrInternal.Code, "failed to set token info to Redis", err)
	}

	return nil
}

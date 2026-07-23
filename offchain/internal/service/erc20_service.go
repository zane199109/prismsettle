package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/zane/web3-offchain/internal/repository"
	"github.com/zane/web3-offchain/internal/storage"
	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/config"
	"github.com/zane/web3-offchain/pkg/constant"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
)

// ERC20Service provides business logic for ERC20 token queries
// including transfer history, balance calculation, and token metadata
type ERC20Service struct {
	ceRepo           *repository.ChainEventRepository
	rds              *storage.RedisStorage
	syncStateService *SyncStateService
}

// NewERC20Service creates a new ERC20Service instance with dependency injection
func NewERC20Service(
	ceRepo *repository.ChainEventRepository,
	rds *storage.RedisStorage,
	syncStateService *SyncStateService,
) *ERC20Service {
	return &ERC20Service{
		ceRepo:           ceRepo,
		rds:              rds,
		syncStateService: syncStateService,
	}
}

// GetTransfers queries paginated ERC20 transfer records with Redis cache
// Uses cache-aside pattern for performance and fault tolerance
func (es *ERC20Service) GetTransfers(
	ctx context.Context,
	chainName, address, tokenAddr string,
	startTime, endTime int64,
	page, size int,
) ([]model.ChainEvent, int64, error) {

	// Validate and sanitize pagination parameters
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20 // Prevent large page attacks
	}

	// Build standardized cache key
	cacheKey := es.buildTransferCacheKey(chainName, address, tokenAddr, startTime, endTime, page, size)

	// Try to get data from cache first
	if cachedData, err := es.rds.Get(ctx, cacheKey); err == nil && cachedData != "" {
		var resp struct {
			List  []model.ChainEvent `json:"list"`
			Total int64              `json:"total"`
		}
		if err := json.Unmarshal([]byte(cachedData), &resp); err == nil {
			return resp.List, resp.Total, nil
		}
		// Clean up corrupted cache
		_ = es.rds.Del(context.Background(), cacheKey)
	}

	// Cache miss: query from database
	list, total, err := es.ceRepo.GetTransfers(ctx, chainName, address, tokenAddr, startTime, endTime, page, size)
	if err != nil {
		logger.Errorf("GetTransfers query db failed", logger.Error(err))
		return nil, 0, errno.ErrInternal
	}

	// Async write to cache (non-blocking)
	go func() {
		ctxAsync, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		resp := struct {
			List  []model.ChainEvent `json:"list"`
			Total int64              `json:"total"`
		}{List: list, Total: total}

		data, _ := json.Marshal(resp)
		_ = es.rds.Set(ctxAsync, cacheKey, string(data), 2*time.Minute)
	}()

	return list, total, nil
}

// GetBalanceWithDetail retrieves balance details for both native coin and ERC20 tokens
// Returns formatted balance, symbol, and decimals
func (es *ERC20Service) GetBalanceWithDetail(
	ctx context.Context,
	chainName, address, tokenAddr string,
) (*model.BalanceVO, error) {

	if address == "" {
		return nil, errno.ErrInvalidParam
	}

	vo := &model.BalanceVO{
		Address:   address,
		ChainName: chainName,
		TokenAddr: tokenAddr,
	}

	var (
		balanceRaw *big.Int
		symbol     string
		decimals   uint8
	)

	// Check if token is native blockchain currency
	isNative := tokenAddr == "" || strings.EqualFold(tokenAddr, constant.NativeTokenAddress)

	if isNative {
		// Native currency symbol/decimals come from chain config (with
		// "ETH"/18 defaults applied in config.Validate). This avoids the
		// hard-coded chain→symbol switch the previous version had, which
		// silently mislabeled any chain not in the list as "ETH".
		symbol, decimals = es.getNativeSymbolAndDecimals(chainName)

		bal, err := es.ceRepo.GetNativeBalance(ctx, chainName, address)
		if err != nil {
			logger.Warn("GetNativeBalance failed", logger.Error(err))
			return nil, errno.ErrInternal
		}
		balanceRaw = bal
	} else {
		// Handle ERC20 token
		tokenInfo, err := es.getTokenInfoSafe(ctx, chainName, tokenAddr)
		if err != nil {
			return nil, err
		}
		symbol = tokenInfo.Symbol
		decimals = tokenInfo.Decimals

		bal, err := es.ceRepo.GetTokenBalance(ctx, chainName, address, tokenAddr)
		if err != nil {
			balanceRaw = new(big.Int)
		} else {
			balanceRaw = bal
		}
	}

	// Populate response with raw and formatted balance
	vo.Symbol = symbol
	vo.Decimals = decimals
	vo.BalanceRaw = balanceRaw.String()
	vo.BalanceFmt = es.formatBalance(balanceRaw, decimals)

	return vo, nil
}

// GetTokenInfo returns metadata for a given ERC20 token (symbol, decimals)
func (es *ERC20Service) GetTokenInfo(ctx context.Context, chainName, tokenAddr string) (*model.TokenInfo, error) {
	if tokenAddr == "" {
		return nil, errno.ErrInvalidParam
	}

	return es.getTokenInfoSafe(ctx, chainName, tokenAddr)
}

// buildTransferCacheKey generates a unique Redis key for transfer list cache
func (es *ERC20Service) buildTransferCacheKey(
	chainName, address, tokenAddr string,
	startTime, endTime int64,
	page, size int,
) string {
	return fmt.Sprintf(
		"transfers:%s:%s:%s:%d:%d:%d:%d",
		chainName,
		address,
		tokenAddr,
		startTime,
		endTime,
		page,
		size,
	)
}

// getTokenInfoSafe retrieves token info with fallback to default values
// Prevents service interruption due to missing metadata
func (es *ERC20Service) getTokenInfoSafe(ctx context.Context, chainName, tokenAddr string) (*model.TokenInfo, error) {
	info, err := es.syncStateService.GetTokenInfo(ctx, chainName, tokenAddr)
	if err == nil && info != nil && info.Symbol != "" {
		return info, nil
	}

	// Fallback to default values to ensure business continuity
	return &model.TokenInfo{
		Symbol:   "UNKNOWN",
		Decimals: 18,
	}, nil
}

// formatBalance converts raw wei balance to human-readable format
// Safe for large numbers and avoids panics
func (es *ERC20Service) formatBalance(balance *big.Int, decimals uint8) string {
	if balance == nil || balance.Sign() == 0 {
		return "0"
	}

	f := new(big.Float).SetInt(balance)
	div := new(big.Float).SetInt(
		new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil),
	)

	res := new(big.Float).Quo(f, div)
	return res.Text('f', -1)
}

// getNativeSymbolAndDecimals returns the chain's native currency symbol and
// decimals from chain config (with "ETH"/18 defaults applied in config.Validate).
// This replaces the previous hard-coded chain→symbol switch, which silently
// mislabeled any chain not in the list as "ETH".
func (es *ERC20Service) getNativeSymbolAndDecimals(chainName string) (string, uint8) {
	if config.Cfg == nil {
		return "ETH", 18
	}
	for _, c := range config.Cfg.Web3.Chains {
		if strings.EqualFold(c.ChainName, chainName) {
			return c.NativeSymbol, uint8(c.NativeDecimals)
		}
	}
	return "ETH", 18
}

package repository

import (
	"context"
	"math/big"
	"strings"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/constant"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
)

// GetTransfers queries paginated ERC20 transfer events
// Supports filtering by chain, address, token, and time range
func (r *ChainEventRepository) GetTransfers(
	ctx context.Context,
	chainName, address, tokenAddr string,
	startTime, endTime int64,
	page, size int,
) ([]model.ChainEvent, int64, error) {

	var list []model.ChainEvent
	var total int64

	// Base query for chain events
	query := r.db.WithContext(ctx).Model(&model.ChainEvent{})

	// Apply query filters
	if chainName != "" {
		query = query.Where("chain_name = ?", chainName)
	}
	if address != "" {
		// LOWER() on both sides so the query matches stored EIP-55 checksum
		// addresses regardless of the casing the API client used. Without
		// this, an all-lowercase input from the handler would silently miss.
		lowerAddr := strings.ToLower(address)
		query = query.Where(`LOWER("from") = ? OR LOWER("to") = ?`, lowerAddr, lowerAddr)
	}
	if tokenAddr != "" {
		// Skip contract filter for native coin
		if !strings.EqualFold(tokenAddr, constant.NativeTokenAddress) {
			query = query.Where("LOWER(contract) = ?", strings.ToLower(tokenAddr))
		}
	}
	if startTime > 0 {
		query = query.Where("block_time >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("block_time <= ?", endTime)
	}

	// Get total count for pagination
	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("repo count transfers failed", logger.Error(err))
		return nil, 0, errno.ErrInternal
	}

	// Query paginated results sorted by block height
	offset := (page - 1) * size
	err := query.
		Order("block_number DESC, log_index DESC").
		Offset(offset).
		Limit(size).
		Find(&list).Error

	if err != nil {
		logger.Errorf("repo query transfers failed", logger.Error(err))
		return nil, 0, errno.ErrInternal
	}

	return list, total, nil
}

// GetTokenBalance calculates ERC20 token balance by summing transfer events
// Balance = total incoming - total outgoing
func (r *ChainEventRepository) GetTokenBalance(
	ctx context.Context,
	chainName, address, tokenAddr string,
) (*big.Int, error) {

	const table = "chain_events"
	var inTotalStr, outTotalStr string

	// Sum of all incoming transfers.
	// LOWER() on "to" so an all-lowercase API input matches the stored
	// EIP-55 checksum address; LOWER() on contract for the same reason.
	lowerAddr := strings.ToLower(address)
	lowerToken := strings.ToLower(tokenAddr)

	inSQL := `
		SELECT COALESCE(SUM(value), '0')
		FROM ` + table + `
		WHERE chain_name = ?
		  AND LOWER("to") = ?
		  AND LOWER(contract) = ?
	`
	if err := r.db.WithContext(ctx).Raw(inSQL, chainName, lowerAddr, lowerToken).Scan(&inTotalStr).Error; err != nil {
		logger.Errorf("repo get token incoming failed", logger.Error(err))
		return new(big.Int), errno.ErrInternal
	}

	// Sum of all outgoing transfers
	outSQL := `
		SELECT COALESCE(SUM(value), '0')
		FROM ` + table + `
		WHERE chain_name = ?
		  AND LOWER("from") = ?
		  AND LOWER(contract) = ?
	`
	if err := r.db.WithContext(ctx).Raw(outSQL, chainName, lowerAddr, lowerToken).Scan(&outTotalStr).Error; err != nil {
		logger.Errorf("repo get token outgoing failed", logger.Error(err))
		return new(big.Int), errno.ErrInternal
	}

	// Parse string values to big.Int
	inTotal := new(big.Int)
	inTotal.SetString(inTotalStr, 10)

	outTotal := new(big.Int)
	outTotal.SetString(outTotalStr, 10)

	// Final balance = incoming - outgoing
	return new(big.Int).Sub(inTotal, outTotal), nil
}

// GetNativeBalance calculates native coin balance (ETH/BNB/MATIC)
// Native transfers are identified by token_addr matching either the zero address
// (0x0000...0000) or the DeFi-standard ETH placeholder (0xEeee...EeeeE).
func (r *ChainEventRepository) GetNativeBalance(
	ctx context.Context,
	chainName, address string,
) (*big.Int, error) {

	const table = "chain_events"
	var inTotalStr, outTotalStr string

	nativeZero := strings.ToLower(constant.NativeTokenAddress)
	nativeETH := strings.ToLower(constant.NativeETHPlaceholder)
	lowerAddr := strings.ToLower(address)

	// Sum incoming native transfers
	inSQL := `
		SELECT COALESCE(SUM(value), '0')
		FROM ` + table + `
		WHERE chain_name = ?
		  AND LOWER("to") = ?
		  AND LOWER(token_addr) IN (?, ?)
	`
	if err := r.db.WithContext(ctx).Raw(inSQL, chainName, lowerAddr, nativeZero, nativeETH).Scan(&inTotalStr).Error; err != nil {
		logger.Errorf("repo get native incoming failed", logger.Error(err))
		return new(big.Int), errno.ErrInternal
	}

	// Sum outgoing native transfers
	outSQL := `
		SELECT COALESCE(SUM(value), '0')
		FROM ` + table + `
		WHERE chain_name = ?
		  AND LOWER("from") = ?
		  AND LOWER(token_addr) IN (?, ?)
	`
	if err := r.db.WithContext(ctx).Raw(outSQL, chainName, lowerAddr, nativeZero, nativeETH).Scan(&outTotalStr).Error; err != nil {
		logger.Errorf("repo get native outgoing failed", logger.Error(err))
		return new(big.Int), errno.ErrInternal
	}

	// Parse string values to big.Int
	inTotal := new(big.Int)
	inTotal.SetString(inTotalStr, 10)

	outTotal := new(big.Int)
	outTotal.SetString(outTotalStr, 10)

	// Final balance = incoming - outgoing
	return new(big.Int).Sub(inTotal, outTotal), nil
}

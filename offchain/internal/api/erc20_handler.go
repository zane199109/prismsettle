package api

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/zane/web3-offchain/internal/service"
	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
	"github.com/zane/web3-offchain/pkg/response"
)

// ERC20Handler processor (dependency injection)
type ERC20Handler struct {
	erc20Service *service.ERC20Service
	validate     *validator.Validate // [Production optimization] Struct injection, not global variable
}

// NewERC20Handler constructor (accepts validator to avoid race conditions)
func NewERC20Handler(erc20Service *service.ERC20Service, validate *validator.Validate) *ERC20Handler {
	if validate == nil {
		validate = validator.New()
	}
	return &ERC20Handler{
		erc20Service: erc20Service,
		validate:     validate,
	}
}

// ------------------------------
// Route registration (standard convention)
// ------------------------------
func (h *ERC20Handler) RegisterRoutes(v1 *gin.RouterGroup) {
	g := v1.Group("/erc20")
	{
		g.GET("/transfers", h.getTransfers)
		g.GET("/balance", h.getBalance)
		g.GET("/token/info", h.getTokenInfo)
	}
}

// ------------------------------
// Request struct (production-grade specification)
// ------------------------------

// TransferQuery transfer query parameters
type TransferQuery struct {
	ChainName string `form:"chainName" binding:"required" validate:"required"`
	Address   string `form:"address" validate:"omitempty,eth_addr"`
	TokenAddr string `form:"tokenAddr" validate:"omitempty,eth_addr"`
	StartTime int64  `form:"startTime" validate:"omitempty,min=0"`
	EndTime   int64  `form:"endTime" validate:"omitempty,min=0"`
	Page      int    `form:"page" default:"1" validate:"required,min=1"`
	Size      int    `form:"size" default:"20" validate:"required,min=1,max=100"`
}

// BalanceQuery balance query parameters
type BalanceQuery struct {
	ChainName string `form:"chainName" binding:"required" validate:"required"`
	Address   string `form:"address" binding:"required" validate:"required,eth_addr"`
	TokenAddr string `form:"tokenAddr" validate:"omitempty,eth_addr"`
}

// TokenInfoQuery token info parameters
type TokenInfoQuery struct {
	ChainName string `form:"chainName" binding:"required" validate:"required"`
	TokenAddr string `form:"tokenAddr" binding:"required" validate:"required,eth_addr"`
}

// ------------------------------
// API endpoints (production-grade implementation)
// ------------------------------

// getTransfers query transfer records
func (h *ERC20Handler) getTransfers(c *gin.Context) {
	var req TransferQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid parameters")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}

	// [Production optimization] Validate time range
	if req.StartTime > 0 && req.EndTime > 0 && req.StartTime > req.EndTime {
		response.Fail(c, errno.ErrInvalidParam.Code, "start time cannot be greater than end time")
		return
	}

	// [Production optimization] Normalize address to lowercase to prevent cache penetration
	req.Address = strings.ToLower(req.Address)
	req.TokenAddr = strings.ToLower(req.TokenAddr)

	start := time.Now()
	list, total, err := h.erc20Service.GetTransfers(
		c.Request.Context(),
		req.ChainName,
		req.Address,
		req.TokenAddr,
		req.StartTime,
		req.EndTime,
		req.Page,
		req.Size,
	)
	if err != nil {
		logger.Warn("get transfers failed",
			logger.String("chain", req.ChainName),
			logger.String("address", req.Address),
			logger.Error(err),
		)
		response.HandleError(c, err)
		return
	}

	// [Production optimization] Avoid returning null, force return empty array
	voList := make([]*model.ERC20TransferVO, 0, len(list))
	for _, item := range list {
		voList = append(voList, model.ConvertToERC20TransferVO(item))
	}

	logger.Info("get transfers success",
		logger.String("chain", req.ChainName),
		logger.Int("count", len(voList)),
		logger.Int64("total", total),
		logger.Cost(start),
	)

	response.Success(c, gin.H{
		"list":  voList,
		"total": total,
		"page":  req.Page,
		"size":  req.Size,
	})
}

// getBalance query balance (native token + ERC20)
func (h *ERC20Handler) getBalance(c *gin.Context) {
	var req BalanceQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid parameters")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}

	// Normalize to lowercase to prevent DB/cache index invalidation
	req.Address = strings.ToLower(req.Address)
	req.TokenAddr = strings.ToLower(req.TokenAddr)

	balanceVO, err := h.erc20Service.GetBalanceWithDetail(
		c.Request.Context(),
		req.ChainName,
		req.Address,
		req.TokenAddr,
	)
	if err != nil {
		logger.Warn("get balance failed",
			logger.String("chain", req.ChainName),
			logger.String("address", req.Address),
			logger.Error(err),
		)
		response.HandleError(c, err)
		return
	}

	logger.Info("get balance success",
		logger.String("chain", req.ChainName),
		logger.String("address", req.Address),
	)
	response.Success(c, balanceVO)
}

// getTokenInfo fetch token info
func (h *ERC20Handler) getTokenInfo(c *gin.Context) {
	var req TokenInfoQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid parameters")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}

	req.TokenAddr = strings.ToLower(req.TokenAddr)

	info, err := h.erc20Service.GetTokenInfo(
		c.Request.Context(),
		req.ChainName,
		req.TokenAddr,
	)
	if err != nil {
		logger.Warn("get token info failed",
			logger.String("token", req.TokenAddr),
			logger.Error(err),
		)
		response.HandleError(c, err)
		return
	}

	vo := model.TokenInfoVO{
		Address:  req.TokenAddr,
		Symbol:   info.Symbol,
		Decimals: info.Decimals,
	}

	response.Success(c, vo)
}

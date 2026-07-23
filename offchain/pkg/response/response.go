package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
)

// Response defines the standard HTTP response structure for all APIs
// Code: 0 means success, non-0 means business error
// Message: human-readable prompt message
// Data: actual business data
type Response struct {
	Code    int         `json:"code"`    // 0 = success, non-0 = error
	Message string      `json:"message"` // response message
	Data    interface{} `json:"data"`    // response payload
}

// Success returns a standard successful response
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Fail returns a standard failed response with custom code and message
func Fail(c *gin.Context, code int, msg string) {
	logger.Errorf("API request failed",
		logger.Int("code", code),
		logger.String("msg", msg),
	)
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: msg,
		Data:    nil,
	})
}

// HandleError uniformly handles errors returned from the service layer
// It recognizes custom errno errors and converts them into standard API responses
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	// If it's a custom business error
	if customErr, ok := err.(*errno.Errno); ok {
		Fail(c, customErr.Code, customErr.Msg)
	} else {
		// For unknown system errors, return generic internal error
		logger.Errorf("Unhandled system error", logger.Error(err))
		Fail(c, errno.ErrInternal.Code, "Internal Server Error")
	}

	// Stop executing subsequent handlers
	c.Abort()
}

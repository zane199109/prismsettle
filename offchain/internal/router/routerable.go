package router

import "github.com/gin-gonic/gin"

// Routable  any handler that wants to register routes must implement this interface
type Routable interface {
	RegisterRoutes(v1 *gin.RouterGroup)
}

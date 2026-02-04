package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery is a middleware that recovers from any panics
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			log.Printf("[PANIC] %s", err)
		} else if err, ok := recovered.(error); ok {
			log.Printf("[PANIC] %v", err)
		} else {
			log.Printf("[PANIC] unknown error: %v", recovered)
		}

		// Log stack trace
		stack := debug.Stack()
		log.Printf("[PANIC] Stack trace:\n%s", stack)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
		c.Abort()
	})
}

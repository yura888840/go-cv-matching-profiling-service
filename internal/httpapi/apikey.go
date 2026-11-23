package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func digest(v string) []byte { h := sha256.Sum256([]byte(v)); return h[:] }

// APIKeyMiddleware requires X-API-Key. A missing configuration fails closed.
// Compare SHA-256 digests to avoid timing differences even for different lengths.
func APIKeyMiddleware(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		supplied := c.GetHeader("X-API-Key")
		if supplied == "" || len(expected) < 32 || subtle.ConstantTimeCompare(digest(strings.TrimSpace(supplied)), digest(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

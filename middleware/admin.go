package middleware

import (
	"net/http"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
)

// 鉴权中间件
func AdminAuthMiddleware() gin.HandlerFunc {
	adminToken := os.Getenv("ADMIN_TOKEN")
	return func(c *gin.Context) {
		token, err := c.Cookie("X-Admin-Token")
		if err != nil || token != adminToken {
			c.Redirect(http.StatusSeeOther, "/login?go="+url.QueryEscape(c.Request.URL.Path))
			c.Abort() //
			return
		}
		c.Next()
	}
}

package middleware

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// AdminAuthMiddleware 全局登录状态记录
func AdminAuthMiddleware() gin.HandlerFunc {
	adminToken := os.Getenv("ADMIN_TOKEN")
	return func(c *gin.Context) {
		token, err := c.Cookie("X-Admin-Token")
		// 判定逻辑：err==nil 表示拿到了 cookie
		isAdmin := err == nil &&
			len(adminToken) > 0 &&
			subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) == 1
		c.Set("isAdmin", isAdmin)
		c.Next()
	}
}

func RequireLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAdmin(c) {
			c.Next()
			return
		}
		// 防止 open redirect：仅允许站内路径
		goURL := c.Request.URL.Path
		if !strings.HasPrefix(goURL, "/") {
			goURL = "/"
		}
		c.Redirect(http.StatusSeeOther, "/login?go="+url.QueryEscape(goURL))
		c.Abort() // 记得终止链路
	}
}
func IsAdmin(c *gin.Context) bool {
	v, ok := c.Get("isAdmin")
	b, _ := v.(bool)
	return ok && b
}

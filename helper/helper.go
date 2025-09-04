package helper

import (
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/gin-gonic/gin"
)

// 统一渲染：自动把 SiteKey 合并到模板数据里
func RenderHTML(c *gin.Context, status int, tmpl string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	data["SiteKey"] = os.Getenv("CF_TURNSTILE_SITEKEY")
	c.HTML(status, tmpl, data)
}
func HashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

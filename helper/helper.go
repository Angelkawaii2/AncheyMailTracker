package helper

import (
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

package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type VerifyFunc func(c *gin.Context, token, ip string) (Result, error)

type Result struct {
	Success bool
	// 需要的话可加更多字段：ChallengeTS, Hostname, ...
}

type TurnstileConfig struct {
	// 表单字段名（Cloudflare 默认是 "cf-turnstile-response"）
	TokenField string
	// 实际的校验函数（注入 services.VerifyTurnstile）
	Verify VerifyFunc
	// 失败时如何响应（可渲染模版或返回 JSON）
	OnFail func(c *gin.Context, err error)
}

// Context 中保存的 key，方便下游读取结果
const CtxTurnstileResult = "turnstile:result"

func TurnstileGuard(cfg TurnstileConfig) gin.HandlerFunc {
	// 合理的默认值
	if cfg.TokenField == "" {
		cfg.TokenField = "cf-turnstile-response"
	}
	if cfg.Verify == nil {
		panic("TurnstileGuard: Verify function is required")
	}
	if cfg.OnFail == nil {
		cfg.OnFail = func(c *gin.Context, err error) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "captcha verification failed"})
		}
	}

	return func(c *gin.Context) {
		token := c.PostForm(cfg.TokenField)
		if token == "" {
			cfg.OnFail(c, errors.New("missing turnstile response"))
			c.Abort()
			return
		}
		v, err := cfg.Verify(c, token, c.ClientIP())
		if err != nil || !v.Success {
			cfg.OnFail(c, err)
			c.Abort()
			return
		}
		// 验证通过，塞进 Context 给下游用
		c.Set(CtxTurnstileResult, v)
		c.Next()
	}
}

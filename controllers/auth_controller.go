package controllers

import (
	"crypto/subtle"
	"mailtrackerProject/helper"
	"mailtrackerProject/middleware"
	"mailtrackerProject/services"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(r *gin.Engine) {
	r.GET("/login", func(c *gin.Context) {
		helper.RenderHTML(c, http.StatusOK, "login.html", gin.H{"Redirect": c.Query("go")})
	})

	r.POST("/login", middleware.TurnstileGuard(middleware.TurnstileConfig{
		Verify: func(c *gin.Context, token, ip string) (middleware.Result, error) {
			res, err := services.VerifyTurnstile(c, token, ip)
			return middleware.Result{Success: err == nil && res.Success}, err
		},
		OnFail: func(c *gin.Context, err error) {
			helper.RenderHTML(c, http.StatusBadRequest, "view_check.html", gin.H{"error": "验证码核验失败，请重试。"})
			return
		},
	}), func(c *gin.Context) {
		password := c.PostForm("password")
		adminToken := os.Getenv("ADMIN_TOKEN")

		target := c.PostForm("redirect")
		if target == "" {
			target = "/"
		}

		if subtle.ConstantTimeCompare([]byte(password), []byte(adminToken)) != 1 {
			helper.RenderHTML(c, http.StatusUnauthorized, "login.html", gin.H{"Error": "账号或密码错误"})
			return
		}

		c.SetCookie("X-Admin-Token", adminToken, 3600*24, "/", "", false, true)
		c.Redirect(http.StatusSeeOther, target)
	})
}

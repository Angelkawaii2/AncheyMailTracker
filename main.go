package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"mailtrackerProject/controllers"
	"mailtrackerProject/services"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func main() {
	dataDir := getEnvDefault("DATA_DIR", "./data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("cannot create DATA_DIR: %v", err)
	}

	adminToken := os.Getenv("ADMIN_TOKEN")
	if adminToken == "" {
		log.Println("[WARN] ADMIN_TOKEN environment variable not set, using random token.")

		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			log.Fatalf("failed to generate random token: %v", err)
		}
		adminToken = hex.EncodeToString(buf)
		err := os.Setenv("ADMIN_TOKEN", adminToken)
		log.Printf("Generated ADMIN_TOKEN: %s\n", adminToken)
		if err != nil {
			log.Fatalf("failed to set ADMIN_TOKEN env: %v", err)
			return
		}
	}

	cfToken := os.Getenv("CF_TURNSTILE_SECRET")
	cfSiteKey := os.Getenv("CF_TURNSTILE_SITEKEY")
	if cfToken == "" || cfSiteKey == "" {
		log.Fatalf("CF_TURNSTILE_SITEKEY or CF_TURNSTILE_SECRET not set")
	}

	// Services
	keysSvc := services.NewKeysService(filepath.Join(dataDir, "keys.json"))
	if err := keysSvc.Load(); err != nil {
		log.Fatalf("load keys: %v", err)
	}

	entriesSvc := services.NewEntriesService(dataDir, keysSvc)
	fileSrvc := services.NewFilesService(dataDir)

	// Router
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Load HTML templates (SSR view kept minimal; APIs return JSON)
	r.LoadHTMLGlob("templates/*.html")
	r.Static("/styles", "./styles")

	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html",
			gin.H{
				"Redirect": c.Query("go"),
			})
	})

	r.POST("/login", func(c *gin.Context) {
		password := c.PostForm("password")
		adminToken := os.Getenv("ADMIN_TOKEN")

		target := c.PostForm("redirect")
		if target == "" {
			target = "/"
		}

		if password != adminToken {
			c.HTML(http.StatusUnauthorized, "login.html", gin.H{
				"Error": "密码错误",
			})
			return
		}
		// 设置 Cookie (HttpOnly，防止 JS 获取)
		c.SetCookie("X-Admin-Token", adminToken, 3600*24, "/", "", false, true)
		// 登录成功后跳转
		c.Redirect(http.StatusSeeOther, target)
	})

	createHandler := func(c *gin.Context) {
		key := c.Param("key")
		c.HTML(http.StatusOK, "create.html", gin.H{
			"Key": key,
		})
	}
	r.GET("/create", authMiddleware(), createHandler)
	r.GET("/create/:key", authMiddleware(), createHandler)

	// 首页
	r.GET("/", func(c *gin.Context) {
		tk, _ := c.Cookie("X-Admin-Token")
		auth := tk == adminToken
		c.HTML(http.StatusOK, "index.html", gin.H{"Authenticated": auth})
	})

	r.GET("/img/:key/:imgName", func(c *gin.Context) {
		// todo 检查origin头
		key := c.Param("key")
		img := c.Param("imgName")

		base := filepath.Join(".", "data", "entries") // . 表示当前工作目录
		abs := filepath.Join(base, key, "images", img)
		log.Printf(abs)
		c.File(abs)
	})

	admin := r.Group("/admin", authMiddleware())
	{
		//admin.GET('/')
		admin.GET("/keys/generate", func(c *gin.Context) {
			c.HTML(http.StatusOK, "key_gen.html", gin.H{})
		})
		admin.POST("/keys/generate", controllers.KeysGenerate(keysSvc))
		admin.GET("/keys/status/:key", controllers.KeyStatus(keysSvc, entriesSvc))
		admin.GET("/keys", controllers.KeysList(keysSvc, entriesSvc))
	}

	api := r.Group("")
	{
		api.GET("/s/:key", controllers.GetEntryRouteView(entriesSvc, keysSvc))
		// Fetch entry data by key (and record UA to history.json if it exists)
		api.POST("/entry", controllers.PostEntry(entriesSvc, fileSrvc, keysSvc))

		siteKey := os.Getenv("CF_TURNSTILE_SITEKEY")

		//视图落地页，没有密码时要求用户输入
		viewCheckHandler := func(c *gin.Context) {
			key := c.Param("key")
			c.HTML(http.StatusOK, "view_check.html", gin.H{"Key": key, "SiteKey": siteKey})
		}
		api.GET("/lookup/", viewCheckHandler)
		api.GET("/lookup/:key", viewCheckHandler)

		//查询表单提交点
		api.POST("/lookup/:key", func(c *gin.Context) {
			key := c.Param("key")
			token := c.PostForm("cf-turnstile-response")
			if token == "" {
				c.String(http.StatusBadRequest, "missing turnstile response")
				return
			}
			//检查验证码
			v, err := services.VerifyTurnstile(c, token, c.ClientIP())
			if err != nil || !v.Success {
				// fail-closed：失败直接回到表单页并提示
				c.HTML(http.StatusBadRequest, "view_check.html", gin.H{"error": "验证码核验失败，请重试。", "SiteKey": siteKey})
				return
			}

			//todo 写完后删掉
			c.JSON(http.StatusOK, gin.H{"test": "verify success!", "key": key, "token": token})

			//检查是否有权限访问

			//访问目标key
			entry := entriesSvc.LoadData(key)
			if entry.Data {

			}

			//写jwt

			return
		})

		//视图实际加载页
		api.GET("/view/:key/:hashedRecipient", controllers.GetEntryView(entriesSvc))
	}

	addr := getEnvDefault("ADDR", ":8080")
	log.Printf("listening on %s (DATA_DIR=%s)", addr, dataDir)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func getEnvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// 鉴权中间件
func authMiddleware() gin.HandlerFunc {
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

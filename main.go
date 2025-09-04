package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"mailtrackerProject/controllers"
	"mailtrackerProject/helper"
	"mailtrackerProject/middleware"
	"mailtrackerProject/models"
	"mailtrackerProject/services"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func main() {
	dataDir := "./data"

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

	//验证码相关设置
	cfToken := os.Getenv("CF_TURNSTILE_SECRET")
	cfSiteKey := os.Getenv("CF_TURNSTILE_SITEKEY")
	if cfToken == "" || cfSiteKey == "" {
		log.Fatalf("CF_TURNSTILE_SITEKEY or CF_TURNSTILE_SECRET not set")
	}

	// Services

	//加载ip库

	cityPath := os.Getenv("CITY_MMDB")
	asnPath := os.Getenv("ASN_MMDB")

	// 2) 初始化单例（geoip2.Reader 是 goroutine-safe，可全局复用）
	geoService, err := services.NewGeoSerice(cityPath, asnPath)
	if err != nil {
		log.Fatalf("init geo service: %v", err)
	}
	defer geoService.Close()

	//加载key service
	keysSvc := services.NewKeysService(filepath.Join(dataDir, "keys.json"))
	if err := keysSvc.Load(); err != nil {
		log.Fatalf("load keys: %v", err)
	}

	entriesSvc := services.NewEntriesService(dataDir, keysSvc)
	fileSrvc := services.NewFilesService(dataDir)

	// Router
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(middleware.AdminAuthMiddleware())

	// Load HTML templates (SSR view kept minimal; APIs return JSON)
	r.LoadHTMLGlob("templates/*.html")
	r.Static("/styles", "./styles")

	r.GET("/login", func(c *gin.Context) {
		helper.RenderHTML(c, http.StatusOK, "login.html", gin.H{"Redirect": c.Query("go")})
	})

	r.GET("/ip", func(c *gin.Context) {
		ip := c.Query("addr")
		if ip == "" {
			ip = c.ClientIP()
		}
		info, err := geoService.Lookup(ip)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "ip": ip})
			return
		}
		fmt.Println(info)
		c.JSON(http.StatusOK, info)
	})

	r.POST("/login", middleware.TurnstileGuard(middleware.TurnstileConfig{
		Verify: func(c *gin.Context, token, ip string) (middleware.Result, error) {
			res, err := services.VerifyTurnstile(c, token, ip)
			return middleware.Result{Success: err == nil && res.Success}, err
		},
		OnFail: func(c *gin.Context, err error) {
			// 失败统一回到验证页（带上 SiteKey）
			fmt.Println(err)
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
			helper.RenderHTML(c, http.StatusUnauthorized, "login.html", gin.H{
				"Error": "账号或密码错误",
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
	r.GET("/create", middleware.RequireLogin(), createHandler)
	r.GET("/create/:key", middleware.RequireLogin(), createHandler)

	// 首页
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{"Authenticated": middleware.IsAdmin(c)})
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

	admin := r.Group("/admin", middleware.RequireLogin())
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
		//二维码落地页
		api.GET("/s/:key", controllers.GetEntryRouteView(entriesSvc, keysSvc))
		//创建表单提交
		api.POST("/entry", controllers.PostEntry(entriesSvc, fileSrvc, keysSvc))

		//查询页，没有密码时要求用户输入
		viewCheckHandler := func(c *gin.Context) {
			key := c.Param("key")
			helper.RenderHTML(c, http.StatusOK, "view_check.html", gin.H{"Key": key})
		}
		api.GET("/lookup/", viewCheckHandler)
		api.GET("/lookup/:key", viewCheckHandler)

		//查询表单提交点
		api.POST("/lookup/", middleware.TurnstileGuard(middleware.TurnstileConfig{
			Verify: func(c *gin.Context, token, ip string) (middleware.Result, error) {
				res, err := services.VerifyTurnstile(c, token, ip)
				return middleware.Result{Success: err == nil && res.Success}, err
			},
			OnFail: func(c *gin.Context, err error) {
				// 失败统一回到验证页（带上 SiteKey）
				fmt.Println(err)
				helper.RenderHTML(c, http.StatusBadRequest, "view_check.html", gin.H{"error": "验证码核验失败，请重试。"})
				return
			},
		}), func(c *gin.Context) {
			key := c.PostForm("keyID")
			formPassword := c.PostForm("formPassword")

			//读取目标key的数据
			entry, err := entriesSvc.LoadData(key)
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "load data failed"})
				c.Abort()
				return
			}

			//鉴权
			encrypt := entry.Data.Encrypt
			if encrypt != nil {
				method := encrypt.Method
				if method != nil {
					//验证收件人
					if *method == "recipient" {
						name := entry.Data.RecipientName
						if name == nil || subtle.ConstantTimeCompare([]byte(formPassword), []byte(*name)) != 1 {
							helper.RenderHTML(c, http.StatusBadRequest, "view_check.html",
								gin.H{"Key": key, "error": "收件人核验失败，请检查输入是否正确（大小写、空格？）"})
							return
						}
					}
					if *method == "password" {
						passwd := encrypt.Password
						if passwd != nil {
							if subtle.ConstantTimeCompare([]byte(*passwd), []byte(formPassword)) != 1 {
								helper.RenderHTML(c, http.StatusBadRequest, "view_check.html",
									gin.H{"Key": key, "error": "密码核验失败，请检查输入是否正确（大小写、空格？）"})

								return
							}
						}
					}
				}
			}

			//todo 检查当前请求时间是否在范围内

			//过鉴权，在这里写日志？
			//todo 不记录管理员查询？（虽然管理员落地也不走这个路由
			//if !middleware.IsAdmin(c) {
			if true {
				// Record UA only if history.json exists for this key
				//todo 记录用户浏览器语言
				ua := c.Request.UserAgent()
				ip := models.ClientIP(c.Request)
				_ = entriesSvc.RecorduaNewlinejson(key, services.HistoryRecord{Time: time.Now(), UA: ua, IP: ip})
			}

			// ========== JWT：读取 -> 解析 -> 追加 -> 回写 ==========
			prevTok := services.ReadTokenFromRequest(c)

			claims, _ := services.ParseClaims(prevTok) // 解析失败也不阻塞；给新 claims

			// 初始化基础字段（如 scope、iat），保持幂等
			if claims.Scope == "" {
				claims.Scope = "page:view"
			}
			if claims.IssuedAt == nil {
				claims.IssuedAt = jwt.NewNumericDate(time.Now())
			}
			claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(90 * 24 * time.Hour))

			// 追加允许访问的 key（去重）
			claims.AllowKeyList = services.AppendAllowKey(claims.AllowKeyList, key)

			// 签发并写回 Cookie
			if err := services.IssueCookie(c, claims); err != nil {
				log.Println("issue jwt error:", err)
				helper.RenderHTML(c, http.StatusInternalServerError, "view_check.html",
					gin.H{"error": "issue jwt error"})
				return
			}

			//写cookie/jwt 跳转到目标页
			c.Redirect(http.StatusSeeOther, "/view/"+key)
			return
		})

		//视图实际加载页
		api.GET("/view/:key/",
			//jwt鉴权中间件
			func(c *gin.Context) {
				key := c.Param("key")
				//非管理才鉴权有无jwt
				if !middleware.IsAdmin(c) {
					tok := services.ReadTokenFromRequest(c)
					claims, err := services.ParseClaims(tok)
					if err != nil || claims == nil {
						helper.RenderHTML(c, http.StatusForbidden, "view_check.html", gin.H{"error": "无访问权限1", "Key": key})
						c.Abort()
						return
					}
					if !slices.Contains(claims.AllowKeyList, key) {
						helper.RenderHTML(c, http.StatusForbidden, "view_check.html", gin.H{"error": "无访问权限2", "Key": key})
						c.Abort()
						return
					}
				}
				c.Next()
			}, controllers.GetEntryView(entriesSvc))
	}

	addr := os.Getenv("PORT")
	log.Printf("listening on %s (DATA_DIR=%s)", addr, dataDir)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

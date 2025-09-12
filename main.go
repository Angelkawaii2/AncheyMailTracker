package main

import (
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"log"
	"mailtrackerProject/controllers"
	"mailtrackerProject/helper"
	"mailtrackerProject/middleware"
	"mailtrackerProject/services"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
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

	logger := helper.NewZap()
	defer logger.Sync()
	// Router
	r := gin.Default()
	// 不带任何中间件的健康检查
	r.GET("/healthy", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.TrustedPlatform = gin.PlatformCloudflare // 读取 CF-Connecting-IP
	r.Use(gin.Recovery(), helper.AccessLogZap(logger))
	r.Use(middleware.AdminAuthMiddleware())

	r.SetFuncMap(template.FuncMap{
		"deref": func(s *string) string {
			if s == nil {
				return ""
			}
			return *s
		},
	})
	// Load HTML templates (SSR view kept minimal; APIs return JSON)
	r.LoadHTMLGlob("templates/*.html")
	r.Static("/styles", "./styles")

	controllers.RegisterAuthRoutes(r)
	controllers.RegisterAdminRoutes(r, keysSvc, entriesSvc)
	controllers.RegisterEntryRoutes(r, entriesSvc, fileSrvc, keysSvc, geoService)

	address := os.Getenv("ADDRESS")
	log.Printf("listening on %s (DATA_DIR=%s)", address, dataDir)
	if err := r.Run(address); err != nil {
		log.Fatal(err)
	}
}

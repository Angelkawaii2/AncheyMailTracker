package controllers

import (
	"crypto/subtle"
	"log"
	"mailtrackerProject/helper"
	"mailtrackerProject/middleware"
	"mailtrackerProject/models"
	"mailtrackerProject/services"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// PostEntry create 路由
func PostEntry(entries *services.EntriesService, files *services.FilesService, keys *services.KeysService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.PostForm("entryId")
		if !models.ValidKey(key) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key format"})
			log.Print("invalid key format: ", key)
			return
		}
		if _, ok := keys.Get(key); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "key not found"})
			return
		}

		recipientName := strings.TrimSpace(c.PostForm("recipientName"))
		remarks := c.PostForm("remarks")
		originLocation := c.PostForm("originLocation")

		postDate := c.PostForm("postDate")
		lookupLimitType := c.PostForm("lookupLimitType")
		lookupLimitAvailableAfterDate := c.PostForm("lookupLimitAvailableAfterDate")
		//enableLookupDate := c.PostForm("enableLookupDate")
		encryptMethod := c.PostForm("encryptMethod")
		encryptPassword := strings.TrimSpace(c.PostForm("encryptPassword"))

		//处理图片上传
		// 解析 multipart 表单并拿到所有同名字段 file
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form"})
			return
		}
		filesFH := form.File["files"]

		if len(filesFH) > 9 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "too many images (max 9)"})
			return
		}

		log.Printf("image count: %d", len(filesFH))

		var imageIDs []string
		for _, fh := range filesFH {
			f, err := fh.Open()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			fileName, err := files.SaveImage(key, f)
			_ = f.Close() // 立即关闭，避免在循环里 defer 堆积
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				log.Print("save image Failed", err)
				return
			}

			imageIDs = append(imageIDs, fileName)
		}

		data := services.EntryData{
			RecipientName:  &recipientName,
			Remarks:        &remarks,
			Images:         &imageIDs,
			OriginLocation: &originLocation,
			PostDate:       &postDate,
		}
		if encryptMethod == "password" {
			if encryptPassword == "" {
				encryptPassword, _ = helper.RandKey(4)
			}
		}

		data.Encrypt = &services.Encrypt{
			Method:   &encryptMethod,
			Password: &encryptPassword,
		}

		data.LookupLimit = &services.LookupLimit{
			Type:           &lookupLimitType,
			AvailableAfter: &lookupLimitAvailableAfterDate,
		}

		if err := entries.SaveData(key, data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		//重定向到目标页面
		c.Redirect(http.StatusSeeOther, "/view/"+key)
	}
}

// GetEntryView view 展示数据页，用中间件鉴权
func GetEntryView(entries *services.EntriesService, service *services.GeoService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		admin := c.GetBool("isAdmin")

		data, err := entries.LoadData(key)
		if err != nil {
			//key不存在
			helper.RenderHTML(c, http.StatusBadRequest, "view_check.html", gin.H{"error": "无效的Key"})
			return
		}

		//获取访问记录
		records, _ := entries.ReadUARecords(key)
		for i := range records {
			records[i].UAObj = helper.ParseUA(records[i].UA)
			records[i].IPObj, _ = service.Lookup(records[i].IP)
			records[i].Timestamp = records[i].Time.UnixMilli()
		}

		if data != nil {
			helper.RenderHTML(c, http.StatusOK, "view.html", gin.H{
				"Key":       key,
				"Admin":     admin,
				"CreatedAt": data.CreatedAt.UnixMilli(),
				"data":      data.Data,
				"records":   records,
			})
			return
		}
		helper.RenderHTML(c, http.StatusInternalServerError, "view_check.html", gin.H{"error": "未知错误"})
	}
}

// GetEntryRouteView s/:key 的路由，在这里跳转创建或查询
func GetEntryRouteView(entries *services.EntriesService, keySrvc *services.KeysService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		//检查key是否位于白名单中，否则认为伪造的key，直接跳转首页
		keyData, ok := keySrvc.Get(key)
		if !ok || keyData.Key == "" {
			log.Printf("Key %s no in allowlist.", key)
			c.Redirect(http.StatusSeeOther, "/")
			return
		}
		//判断key是否创建
		if entries.HasData(key) { //创建了跳转到展示页
			c.Redirect(303, "/lookup/"+key)
		} else { //没创建跳转到创建页
			if middleware.IsAdmin(c) {
				//是管理员就跳转，鉴权由create的中间件处理
				c.Redirect(303, "/create/"+key)
			} else {
				//提示该key未启用，是否现在启用
				c.HTML(http.StatusOK, "key_not_enable.html", gin.H{"Key": key})
			}
		}
	}
}

func PostLookupHandler(entries *services.EntriesService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.PostForm("keyID")
		formPassword := c.PostForm("formPassword")

		//读取目标key的数据
		entry, err := entries.LoadData(key)
		if err != nil {
			//转全大写再查一次
			entry, err = entries.LoadData(strings.ToUpper(key))
		}
		if err != nil {
			log.Println(err)
			//无数据跳转回首页
			helper.RenderHTML(c, http.StatusBadRequest, "view_check.html",
				gin.H{"Key": key, "error": "ID不存在，请检查输入是否有误"},
			)
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

		//过鉴权，在这里写日志？
		//不记录管理员查询 todo 可以改成表单
		if !middleware.IsAdmin(c) {
			// Record UA only if history.json exists for this key
			ua := c.Request.UserAgent()
			ip := c.ClientIP()
			_ = entries.RecorduaNewlinejson(key, services.HistoryRecord{Time: time.Now(), UA: ua, IP: ip})
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
	}
}

func RegisterEntryRoutes(r *gin.Engine,
	entriesSvc *services.EntriesService,
	fileSvc *services.FilesService,
	keysSvc *services.KeysService,
	geoSvc *services.GeoService,
) {
	// create 页面
	createHandler := func(c *gin.Context) {
		key := c.Param("key")
		c.HTML(http.StatusOK, "create.html", gin.H{"Key": key})
	}
	r.GET("/create", middleware.RequireLogin(), createHandler)
	r.GET("/create/:key", middleware.RequireLogin(), createHandler)

	// 首页
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{"Authenticated": middleware.IsAdmin(c)})
	})

	// 图片
	r.GET("/img/:key/:imgName", func(c *gin.Context) {
		key := c.Param("key")
		img := c.Param("imgName")
		base := filepath.Join(".", "data", "entries")
		abs := filepath.Join(base, key, "images", img)
		c.File(abs)
	})

	//二维码 短链落地页
	r.GET("/s/:key", GetEntryRouteView(entriesSvc, keysSvc))
	//创建表单提交
	r.POST("/entry", PostEntry(entriesSvc, fileSvc, keysSvc))

	//查询页，没有密码时要求用户输入
	viewCheckHandler := func(c *gin.Context) {
		key := c.Param("key")
		//读取目标key的数据
		entry, _ := entriesSvc.LoadData(key)
		if entry != nil {
			if entry.Data.Encrypt.Method != nil {
				if *entry.Data.Encrypt.Method == "recipient" {
					helper.RenderHTML(c, http.StatusOK, "view_check.html", gin.H{"Key": key, "EncryptType": "recipient"})
					return
				}
			}
		}
		helper.RenderHTML(c, http.StatusOK, "view_check.html", gin.H{"Key": key})
	}
	r.GET("/lookup/", viewCheckHandler)
	r.GET("/lookup/:key", viewCheckHandler)

	//查询表单提交点
	r.POST("/lookup/", middleware.TurnstileGuard(middleware.TurnstileConfig{
		Verify: func(c *gin.Context, token, ip string) (middleware.Result, error) {
			res, err := services.VerifyTurnstile(c, token, ip)
			return middleware.Result{Success: err == nil && res.Success}, err
		},
		OnFail: func(c *gin.Context, err error) {
			// 失败统一回到验证页（带上 SiteKey）
			helper.RenderHTML(c, http.StatusBadRequest, "view_check.html", gin.H{"error": "验证码核验失败，请重试。"})
			return
		}}), PostLookupHandler(entriesSvc))

	//视图实际加载页
	r.GET("/view/:key/",
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
		}, GetEntryView(entriesSvc, geoSvc))

}

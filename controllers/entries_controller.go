package controllers

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"mailtrackerProject/helper"
	"mailtrackerProject/models"
	"mailtrackerProject/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

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

		recipientName := c.PostForm("recipientName")
		remarks := c.PostForm("remarks")
		originLocation := c.PostForm("originLocation")
		postDate := c.PostForm("postDate")

		// 解析 multipart 表单并拿到所有同名字段 file
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form"})
			return
		}
		filesFH := form.File["files"]
		if len(filesFH) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
			return
		}
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

			fileName, err := files.SaveImage(key, f, true)
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

		if err := entries.SaveData(key, data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		//todo 重定向到目标页面
		c.Redirect(http.StatusSeeOther, "/view/"+key)
	}
}

// view 展示数据页，用中间件鉴权
func GetEntryView(entries *services.EntriesService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		//注入页面用

		data, err := entries.LoadData(key)
		if err != nil {
			//key不存在
			helper.RenderHTML(c, http.StatusBadRequest, "view_check.html", gin.H{"error": "无效的Key"})
			return
		}

		if data != nil {
			helper.RenderHTML(c, http.StatusOK, "view.html", gin.H{
				"Key":       key,
				"CreatedAt": data.CreatedAt.Format("2006-01-02 15:04:05"),
				"data":      data.Data,
			})
			return
		}
		helper.RenderHTML(c, http.StatusInternalServerError, "view_check.html", gin.H{"error": "未知错误"})
	}
}

// s/:key 的路由，在这里跳转创建或查询
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
			//todo create的controller应提示还没启用，要启用再跳转到登录页
			c.Redirect(303, "/create/"+key)
		}
	}
}
func HashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

package controllers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"mailtrackerProject/models"
	"mailtrackerProject/services"
	"net/http"
	"net/url"
	"time"

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

		payload := map[string]any{
			"recipientName":  recipientName,
			"remarks":        remarks,
			"images":         imageIDs, // 由单图 image → 多图 images
			"postDate":       postDate,
			"originLocation": originLocation,
		}
		raw, _ := json.Marshal(payload)
		if err := entries.SaveData(key, raw); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// redirect to view with log=false
		loc := "/view/" + url.PathEscape(key) + "/" + HashString(recipientName)
		c.Redirect(http.StatusSeeOther, loc)
	}
}

func GetEntryView(entries *services.EntriesService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		hashedRecipient := c.Param("hashedRecipient")
		//todo 检测cf验证码

		data, err := entries.LoadData(key)
		if err != nil {
			//key不存在
			//c.JSON(http.StatusOK, gin.H{"error": err})
			c.HTML(http.StatusBadRequest, "view_check.html", gin.H{"error": "无效的Key"})
			return
		}

		if data != nil {

			rawJson, _ := json.Marshal(data)
			var parsedJsonData map[string]any
			if err := json.Unmarshal(data.Data, &parsedJsonData); err != nil {
				log.Print(err)
			}

			name := parsedJsonData["recipientName"].(string)
			hashedName := HashString(name)
			//校验失败，重定向回验证页
			if hashedRecipient != hashedName {
				log.Println(hashedRecipient)
				log.Println(hashedName)
				c.HTML(http.StatusBadRequest, "view_check.html", gin.H{"error": "收件人错误", "Key": key})
				return
			}

			doLog := c.Query("log")
			if doLog == "true" {
				// Record UA only if history.json exists for this key
				ua := c.Request.UserAgent()
				ip := models.ClientIP(c.Request)
				_ = entries.RecordUAIfHistoryExists(key, services.HistoryRecord{Time: time.Now(), UA: ua, IP: ip})
			}

			c.HTML(http.StatusOK, "view.html", gin.H{
				"Key":       key,
				"CreatedAt": data.CreatedAt.Format("2006-01-02 15:04:05"),
				"data":      parsedJsonData,
				"raw":       string(rawJson),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no data no error"})
		}
	}
}

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
		//todo 读取收件人cookie，如果已经存在了就直接加上hash过的收件人，直接进数据页，否则要求先输入一次

		if entries.HasData(key) { //创建了跳转到展示页
			c.Redirect(303, "/lookup/"+key)
		} else { //没创建跳转到创建页
			c.Redirect(303, "/create/"+key)
		}
	}
}
func HashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

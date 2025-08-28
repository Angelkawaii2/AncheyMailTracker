package controllers

import (
	"log"
	"mailtrackerProject/services"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func KeysGenerate(keys *services.KeysService) gin.HandlerFunc {

	return func(c *gin.Context) {

		q, err := strconv.Atoi(c.PostForm("quantity"))
		length, err := strconv.Atoi(c.PostForm("length"))

		if q <= 0 || q > 1000000 {
			c.HTML(http.StatusBadRequest, "key_gen.html", gin.H{"error": "invalid count"})
			return
		}
		if length < 6 || length > 100 {
			c.HTML(http.StatusBadRequest, "key_gen.html", gin.H{"error": "length must be >6 and <100"})
			return
		}

		out, err := keys.Generate(q, length)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		ids := make([]string, 0, len(out))
		for _, item := range out {
			log.Println(item.Key)
			ids = append(ids, item.Key)
		}

		// 转成模板专用结构
		type EntryView struct {
			Key       string
			CreatedAt string
		}
		views := make([]EntryView, len(out))
		for i, e := range out {
			views[i] = EntryView{
				Key:       e.Key,
				CreatedAt: e.CreatedAt.Format("2006-01-02 15:04:05"),
			}
		}

		c.HTML(http.StatusOK, "key_gen.html", gin.H{"keys": views, "ids": ids})
	}
}

// GET /admin/keys/status/:key
func KeyStatus(keys *services.KeysService, entries *services.EntriesService) gin.HandlerFunc {
	return func(c *gin.Context) {
		k := c.Param("key")
		info, ok := keys.Get(k)
		if !ok {
			c.JSON(http.StatusOK, gin.H{"key": k, "status": "not_found"})
			return
		}
		used := entries.HasData(k)
		s := "available"
		if used {
			s = "used"
		}
		c.JSON(http.StatusOK, gin.H{"key": k, "status": s, "created_at": info.CreatedAt.Format(time.RFC3339)})
	}
}

// GET /admin/keys
func KeysList(keys *services.KeysService, entries *services.EntriesService) gin.HandlerFunc {
	return func(c *gin.Context) {
		all := keys.List()
		// annotate used flag by checking data existence
		out := make([]gin.H, 0, len(all))
		for _, ki := range all {
			out = append(out, gin.H{
				"key":        ki.Key,
				"created_at": ki.CreatedAt.Format("2006-01-02 15:04:05"),
				"used":       entries.HasData(ki.Key),
			})
		}
		log.Print(out)
		c.HTML(http.StatusOK, "key_view.html", out)
	}
}

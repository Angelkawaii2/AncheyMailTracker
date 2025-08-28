package controllers

import (
	"log"
	"mailtrackerProject/services"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func KeysGenerate(keys *services.KeysService) gin.HandlerFunc {
	type req struct {
		Count  int `json:"count"`
		Length int `json:"length"`
	}
	return func(c *gin.Context) {
		var r req
		if err := c.ShouldBindJSON(&r); err != nil || r.Count <= 0 || r.Count > 10000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid count"})
			return
		}
		out, err := keys.Generate(r.Count, r.Length)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"keys": out})
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
		//todo 优化展示模板
		//c.JSON(http.StatusOK, gin.H{"items": out})
		c.HTML(http.StatusOK, "key_view.html", out)
	}
}

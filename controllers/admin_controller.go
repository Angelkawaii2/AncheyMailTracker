package controllers

import (
	"mailtrackerProject/middleware"
	"mailtrackerProject/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterAdminRoutes(r *gin.Engine, keysSvc *services.KeysService, entriesSvc *services.EntriesService) {
	admin := r.Group("/admin", middleware.RequireLogin())
	{
		admin.GET("/keys/generate", func(c *gin.Context) {
			c.HTML(http.StatusOK, "key_gen.html", gin.H{})
		})
		admin.POST("/keys/generate", KeysGenerate(keysSvc))
		admin.GET("/keys/status/:key", KeyStatus(keysSvc, entriesSvc))
		admin.GET("/keys", KeysList(keysSvc, entriesSvc))
	}
}

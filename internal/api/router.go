package api

import (
	"m3u8-go/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册所有API路由
func RegisterRoutes(r *gin.Engine) {
	// API 路由组
	api := r.Group("/api")
	{
		// 下载相关路由
		api.POST("/download", handlers.CreateDownload)

		// 任务管理相关路由
		api.GET("/tasks", handlers.GetAllTasks)
		api.GET("/tasks/:id", handlers.GetTaskByID)
		api.POST("/tasks/:id/resume", handlers.ResumeTask)
		api.POST("/tasks/:id/retry", handlers.RetryTask)
		api.POST("/tasks/clear-completed", handlers.ClearCompletedTasks)
		api.DELETE("/tasks/:id", handlers.DeleteTask)

		// 设置相关路由
		api.GET("/settings", handlers.GetSettings)
		api.POST("/settings", handlers.SaveSettings)

		// 文件夹相关路由
		api.GET("/folders", handlers.GetFolders)
		api.POST("/folders/create", handlers.CreateFolder)
	}
}

package handlers

import (
	"m3u8-go/internal/config"
	"m3u8-go/internal/dl"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// GetSettings 获取设置
func GetSettings(c *gin.Context) {
	settings := config.Get()
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "获取配置成功",
		Data:    settings,
	})
}

// SaveSettings 保存设置
func SaveSettings(c *gin.Context) {
	var settings config.Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	// 验证设置数据
	if settings.DefaultOutputPath == "" {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Message: "默认下载位置不能为空",
		})
		return
	}

	if settings.DefaultThreadCount <= 0 || settings.DefaultThreadCount > 128 {
		settings.DefaultThreadCount = config.Get().DefaultThreadCount // 使用默认值
	}

	// 尝试创建目录，检查是否有权限
	if err := os.MkdirAll(settings.DefaultOutputPath, 0755); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Message: "无法创建下载目录，请检查路径和权限: " + err.Error(),
		})
		return
	}

	// 验证同时下载数量
	if settings.MaxConcurrentDownload <= 0 || settings.MaxConcurrentDownload > 10 {
		settings.MaxConcurrentDownload = config.Get().MaxConcurrentDownload // 设置默认值
	}

	// 验证下载速度限制
	if settings.DownloadSpeedLimit < 0 {
		settings.DownloadSpeedLimit = 0 // 负数设为0，表示不限速
	}

	// 更新任务管理器的最大并发下载数和速度限制
	taskManager := dl.GetTaskManager()
	taskManager.UpdateMaxConcurrentDownloads(settings.MaxConcurrentDownload)
	taskManager.UpdateDownloadSpeedLimit(settings.DownloadSpeedLimit)

	// 保存设置
	if err := config.Save(settings); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Message: "保存配置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "配置保存成功",
	})
}

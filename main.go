package main

import (
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"m3u8-go/internal/api"
	"m3u8-go/internal/config"
	"m3u8-go/internal/dl"
	"m3u8-go/internal/tool"

	"github.com/gin-gonic/gin"
)

func main() {
	// 注册退出信号处理，确保清理临时文件
	setupCleanupHandler()

	// 加载配置并初始化任务管理器
	settings, _ := config.Load()
	taskManager := dl.GetTaskManager()
	if settings.MaxConcurrentDownload > 0 && settings.MaxConcurrentDownload <= 10 {
		taskManager.UpdateMaxConcurrentDownloads(settings.MaxConcurrentDownload)
	}

	// 设置下载速度限制 - 这里会调用新的全局限速配置
	if settings.DownloadSpeedLimit >= 0 {
		// 通过 taskManager 设置限速，它内部会调用 tool.ConfigureGlobalRateLimiter
		taskManager.UpdateDownloadSpeedLimit(settings.DownloadSpeedLimit)
		tool.Info("[启动] 初始化全局限速为 %d KB/s", settings.DownloadSpeedLimit)
	} else {
		tool.Info("[启动] 全局限速未设置或无效，无限速")
	}

	// 使用 gin.New() 替代 gin.Default() 以关闭默认日志
	r := gin.New()
	// 只添加 Recovery 中间件，不添加 Logger 中间件
	r.Use(gin.Recovery())

	// 注册API路由
	api.RegisterRoutes(r)

	// 确保静态资源目录存在
	if err := os.MkdirAll("./web/dist", 0755); err != nil {
		panic("无法创建静态资源目录: " + err.Error())
	}

	// 设置静态资源路径
	r.Static("/assets", "./web/dist/assets")
	r.StaticFile("/favicon.ico", "./web/dist/favicon.ico")
	r.StaticFile("/robots.txt", "./web/dist/robots.txt")

	// 所有其他路由返回index.html
	r.GET("/", func(c *gin.Context) {
		c.File("./web/dist/index.html")
	})

	r.NoRoute(func(c *gin.Context) {
		// 如果请求的是静态资源但不存在，返回404
		if filepath.Ext(c.Request.URL.Path) != "" {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		// 否则返回index.html（支持SPA前端路由）
		c.File("./web/dist/index.html")
	})

	r.Run(":9100")
}

// setupCleanupHandler 设置程序退出时的清理处理
func setupCleanupHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		tool.Info("程序退出，正在清理临时文件...")
		tool.Cleanup()
		os.Exit(0)
	}()
}

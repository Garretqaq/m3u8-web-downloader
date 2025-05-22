package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"m3u8-go/internal/config"
	"m3u8-go/internal/dl"
	"m3u8-go/internal/tool"

	"github.com/gin-gonic/gin"
)

type DownloadRequest struct {
	Url            string `json:"url" binding:"required"`
	Output         string `json:"output" binding:"required"`
	C              int    `json:"c"`
	CustomFileName string `json:"customFileName"`
	DeleteTs       bool   `json:"deleteTs"`
	ConvertToMp4   bool   `json:"convertToMp4"`
}

type DownloadResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// TaskInfo 用于API返回的任务信息
type TaskInfo struct {
	ID       string  `json:"id"`       // 任务ID
	URL      string  `json:"url"`      // 下载链接
	Output   string  `json:"output"`   // 输出路径
	C        int     `json:"c"`        // 线程数
	Progress int     `json:"progress"` // 下载进度 (0-100)
	Status   string  `json:"status"`   // 任务状态
	Message  string  `json:"message"`  // 状态信息
	Created  int64   `json:"created"`  // 创建时间
	FileName string  `json:"fileName"` // 输出文件名
	Speed    float64 `json:"speed"`    // 下载速度（字节/秒）
}

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

	// API 路由组
	api := r.Group("/api")
	{
		// 创建下载任务
		api.POST("/download", func(c *gin.Context) {
			var req DownloadRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, DownloadResponse{false, "参数错误: " + err.Error(), nil})
				return
			}
			if req.C <= 0 {
				req.C = config.Get().DefaultThreadCount
			}

			downloader, err := dl.NewTask(req.Output, req.Url)
			if err != nil {
				c.JSON(http.StatusInternalServerError, DownloadResponse{false, "创建下载任务失败: " + err.Error(), nil})
				return
			}

			// 设置用户指定的线程数
			downloader.C = req.C

			// 设置自定义文件名（如果有）
			if req.CustomFileName != "" {
				// 先从任务管理器中删除原文件名占用
				taskManager := dl.GetTaskManager()
				taskManager.DeleteTask(downloader.ID)

				// 设置新文件名，确保有适当的扩展名
				customFileName := req.CustomFileName

				// 获取原始文件名的基础部分（不含扩展名）
				baseFileName := strings.TrimSuffix(customFileName, filepath.Ext(customFileName))

				// 根据是否需要转换为MP4，确定正确的扩展名
				fileExt := ".ts"
				if req.ConvertToMp4 {
					fileExt = ".mp4"
				}

				// 如果用户提供的文件名没有扩展名，或者扩展名不是我们期望的，则添加正确的扩展名
				if filepath.Ext(customFileName) == "" ||
					(req.ConvertToMp4 && !strings.HasSuffix(strings.ToLower(customFileName), ".mp4")) ||
					(!req.ConvertToMp4 && !strings.HasSuffix(strings.ToLower(customFileName), ".ts")) {
					customFileName = baseFileName + fileExt
				}

				// 生成唯一文件名，避免覆盖已有文件
				uniqueFileName := taskManager.GenerateUniqueFileName(downloader.Output, customFileName)
				downloader.FileName = uniqueFileName

				// 重新添加任务到管理器，以更新文件名占用
				taskManager.AddTask(downloader)
			} else if req.ConvertToMp4 {
				// 如果没有指定自定义文件名，但需要转换为MP4
				taskManager := dl.GetTaskManager()
				taskManager.DeleteTask(downloader.ID)

				// 获取原始文件名的基础部分（不含扩展名）
				baseFileName := strings.TrimSuffix(downloader.FileName, filepath.Ext(downloader.FileName))
				mp4FileName := baseFileName + ".mp4"

				// 生成唯一文件名，避免覆盖已有文件
				uniqueFileName := taskManager.GenerateUniqueFileName(downloader.Output, mp4FileName)
				downloader.FileName = uniqueFileName

				taskManager.AddTask(downloader)
			}

			// 设置是否删除分片
			downloader.DeleteTs = req.DeleteTs

			// 设置是否转换为MP4
			downloader.ConvertToMp4 = req.ConvertToMp4

			// 将任务加入下载队列
			taskManager := dl.GetTaskManager()
			taskManager.EnqueueDownload(downloader)

			// 立即返回任务信息
			c.JSON(http.StatusOK, DownloadResponse{true, "下载任务已创建", TaskInfo{
				ID:       downloader.ID,
				URL:      downloader.URL,
				Output:   downloader.Output,
				C:        req.C,
				Progress: downloader.Progress,
				Status:   downloader.Status,
				Message:  downloader.Message,
				Created:  downloader.Created,
				FileName: downloader.FileName,
				Speed:    downloader.Speed,
			}})
		})

		// 获取所有任务
		api.GET("/tasks", func(c *gin.Context) {
			taskManager := dl.GetTaskManager()
			tasks := taskManager.GetAllTasks()

			// 转换为API格式
			taskInfos := make([]TaskInfo, 0, len(tasks))
			for _, task := range tasks {
				taskInfos = append(taskInfos, TaskInfo{
					ID:       task.ID,
					URL:      task.URL,
					Output:   task.Output,
					C:        task.C,
					Progress: task.Progress,
					Status:   task.Status,
					Message:  task.Message,
					Created:  task.Created,
					FileName: task.FileName,
					Speed:    task.Speed,
				})
			}

			c.JSON(http.StatusOK, DownloadResponse{true, "获取任务列表成功", taskInfos})
		})

		// 获取任务详情
		api.GET("/tasks/:id", func(c *gin.Context) {
			id := c.Param("id")
			taskManager := dl.GetTaskManager()
			task := taskManager.GetTask(id)

			if task == nil {
				c.JSON(http.StatusNotFound, DownloadResponse{false, "任务不存在", nil})
				return
			}

			c.JSON(http.StatusOK, DownloadResponse{true, "获取任务成功", TaskInfo{
				ID:       task.ID,
				URL:      task.URL,
				Output:   task.Output,
				C:        task.C,
				Progress: task.Progress,
				Status:   task.Status,
				Message:  task.Message,
				Created:  task.Created,
				FileName: task.FileName,
				Speed:    task.Speed,
			}})
		})

		// 继续下载任务
		api.POST("/tasks/:id/resume", func(c *gin.Context) {
			id := c.Param("id")
			taskManager := dl.GetTaskManager()
			task := taskManager.GetTask(id)

			if task == nil {
				c.JSON(http.StatusNotFound, DownloadResponse{false, "任务不存在", nil})
				return
			}

			if success := task.Resume(); success {
				c.JSON(http.StatusOK, DownloadResponse{true, "任务已继续下载", nil})
			} else {
				c.JSON(http.StatusBadRequest, DownloadResponse{false, "任务无法继续", nil})
			}
		})

		// 清除已完成的下载任务
		api.POST("/tasks/clear-completed", func(c *gin.Context) {
			taskManager := dl.GetTaskManager()
			count := taskManager.ClearCompletedTasks()

			message := fmt.Sprintf("已清除%d个已完成的下载任务", count)
			c.JSON(http.StatusOK, DownloadResponse{true, message, nil})
		})

		// 删除任务
		api.DELETE("/tasks/:id", func(c *gin.Context) {
			id := c.Param("id")
			taskManager := dl.GetTaskManager()

			success, err := taskManager.StopAndDeleteTask(id)
			if !success {
				c.JSON(http.StatusNotFound, DownloadResponse{false, "任务不存在", nil})
				return
			}

			if err != nil {
				c.JSON(http.StatusInternalServerError, DownloadResponse{false, "删除任务文件失败: " + err.Error(), nil})
				return
			}

			c.JSON(http.StatusOK, DownloadResponse{true, "删除任务成功", nil})
		})

		// 获取设置
		api.GET("/settings", func(c *gin.Context) {
			settings := config.Get()
			c.JSON(http.StatusOK, DownloadResponse{
				Success: true,
				Message: "获取配置成功",
				Data:    settings,
			})
		})

		// 保存设置
		api.POST("/settings", func(c *gin.Context) {
			var settings config.Settings
			if err := c.ShouldBindJSON(&settings); err != nil {
				c.JSON(http.StatusBadRequest, DownloadResponse{
					Success: false,
					Message: "参数错误: " + err.Error(),
				})
				return
			}

			// 验证设置数据
			if settings.DefaultOutputPath == "" {
				c.JSON(http.StatusBadRequest, DownloadResponse{
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
				c.JSON(http.StatusBadRequest, DownloadResponse{
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
				c.JSON(http.StatusInternalServerError, DownloadResponse{
					Success: false,
					Message: "保存配置失败: " + err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, DownloadResponse{
				Success: true,
				Message: "配置保存成功",
			})
		})
	}

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
		c.File("./static/index.html")
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

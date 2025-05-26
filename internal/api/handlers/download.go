package handlers

import (
	"m3u8-go/internal/config"
	"m3u8-go/internal/dl"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// CreateDownload 创建下载任务
func CreateDownload(c *gin.Context) {
	var req DownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{false, "参数错误: " + err.Error(), nil})
		return
	}
	if req.C <= 0 {
		req.C = config.Get().DefaultThreadCount
	}

	downloader, err := dl.NewTask(req.Output, req.Url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{false, "创建下载任务失败: " + err.Error(), nil})
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
	c.JSON(http.StatusOK, Response{true, "下载任务已创建", TaskInfo{
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
}

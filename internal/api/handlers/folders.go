package handlers

import (
	"m3u8-go/internal/tool"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetFolders 获取文件夹列表
func GetFolders(c *gin.Context) {
	targetPath := c.Query("path")

	folderList, err := tool.GetFolderList(targetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Message: "获取文件夹列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "获取文件夹列表成功",
		Data:    folderList,
	})
}

// CreateFolder 创建新文件夹
func CreateFolder(c *gin.Context) {
	var req CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	// 验证路径安全性
	if err := tool.ValidatePath(req.Path); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Message: "路径验证失败: " + err.Error(),
		})
		return
	}

	// 创建文件夹
	if err := tool.CreateFolder(req.Path, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Message: "创建文件夹失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "文件夹创建成功",
	})
}

package handlers

import (
	"fmt"
	"m3u8-go/internal/dl"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetAllTasks 获取所有任务
func GetAllTasks(c *gin.Context) {
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

	c.JSON(http.StatusOK, Response{true, "获取任务列表成功", taskInfos})
}

// GetTaskByID 获取任务详情
func GetTaskByID(c *gin.Context) {
	id := c.Param("id")
	taskManager := dl.GetTaskManager()
	task := taskManager.GetTask(id)

	if task == nil {
		c.JSON(http.StatusNotFound, Response{false, "任务不存在", nil})
		return
	}

	c.JSON(http.StatusOK, Response{true, "获取任务成功", TaskInfo{
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
}

// ResumeTask 继续下载任务
func ResumeTask(c *gin.Context) {
	id := c.Param("id")
	taskManager := dl.GetTaskManager()
	task := taskManager.GetTask(id)

	if task == nil {
		c.JSON(http.StatusNotFound, Response{false, "任务不存在", nil})
		return
	}

	if success := task.Resume(); success {
		c.JSON(http.StatusOK, Response{true, "任务已继续下载", nil})
	} else {
		c.JSON(http.StatusBadRequest, Response{false, "任务无法继续", nil})
	}
}

// ClearCompletedTasks 清除已完成的下载任务
func ClearCompletedTasks(c *gin.Context) {
	taskManager := dl.GetTaskManager()
	count := taskManager.ClearCompletedTasks()

	message := fmt.Sprintf("已清除%d个已完成的下载任务", count)
	c.JSON(http.StatusOK, Response{true, message, nil})
}

// DeleteTask 删除任务
func DeleteTask(c *gin.Context) {
	id := c.Param("id")
	taskManager := dl.GetTaskManager()

	success, err := taskManager.StopAndDeleteTask(id)
	if !success {
		c.JSON(http.StatusNotFound, Response{false, "任务不存在", nil})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{false, "删除任务文件失败: " + err.Error(), nil})
		return
	}

	c.JSON(http.StatusOK, Response{true, "删除任务成功", nil})
}

// RetryTask 重试下载任务
func RetryTask(c *gin.Context) {
	id := c.Param("id")
	taskManager := dl.GetTaskManager()
	task := taskManager.GetTask(id)

	if task == nil {
		c.JSON(http.StatusNotFound, Response{false, "任务不存在", nil})
		return
	}

	// 先停止并删除当前任务，然后重新入队
	if success, _ := taskManager.StopAndDeleteTask(id); !success {
		c.JSON(http.StatusBadRequest, Response{false, "无法停止当前任务", nil})
		return
	}

	// 创建新任务并入队
	newTask, err := dl.NewTask(task.Output, task.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{false, "创建新任务失败: " + err.Error(), nil})
		return
	}

	// 复制原任务的相关设置
	newTask.C = task.C
	newTask.DeleteTs = task.DeleteTs
	newTask.ConvertToMp4 = task.ConvertToMp4
	newTask.FileName = task.FileName

	// 将任务加入下载队列
	taskManager.EnqueueDownload(newTask)

	c.JSON(http.StatusOK, Response{true, "任务已重新开始下载", nil})
}

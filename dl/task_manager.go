package dl

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// TaskManager 管理所有下载任务
type TaskManager struct {
	lock        sync.RWMutex
	tasks       map[string]*Downloader // 使用任务ID作为key
	fileNameMap map[string]bool        // 记录已被占用的文件名，格式: "文件夹路径:文件名"
}

// 单例模式
var (
	instance *TaskManager
	once     sync.Once
)

// GetTaskManager 获取任务管理器实例
func GetTaskManager() *TaskManager {
	once.Do(func() {
		instance = &TaskManager{
			tasks:       make(map[string]*Downloader),
			fileNameMap: make(map[string]bool),
		}
	})
	return instance
}

// AddTask 添加任务到管理器
func (tm *TaskManager) AddTask(task *Downloader) {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	tm.tasks[task.ID] = task

	// 标记文件名已被占用
	fileKey := tm.getFileKey(task.Output, task.FileName)
	tm.fileNameMap[fileKey] = true
}

// GetTask 根据ID获取任务
func (tm *TaskManager) GetTask(id string) *Downloader {
	tm.lock.RLock()
	defer tm.lock.RUnlock()
	return tm.tasks[id]
}

// GetAllTasks 获取所有任务
func (tm *TaskManager) GetAllTasks() []*Downloader {
	tm.lock.RLock()
	defer tm.lock.RUnlock()

	result := make([]*Downloader, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		result = append(result, task)
	}

	// 按创建时间排序，最新的任务优先（降序）
	sort.Slice(result, func(i, j int) bool {
		return result[i].Created > result[j].Created
	})

	return result
}

// StopAndDeleteTask 停止任务下载并删除任务文件
func (tm *TaskManager) StopAndDeleteTask(id string) (bool, error) {
	tm.lock.Lock()

	task, exists := tm.tasks[id]
	if !exists {
		tm.lock.Unlock()
		return false, nil
	}

	// 取消文件名占用
	fileKey := tm.getFileKey(task.Output, task.FileName)
	delete(tm.fileNameMap, fileKey)

	// 先解锁，因为 Stop 方法内部会获取锁
	tm.lock.Unlock()

	// 1. 停止任务下载
	fmt.Printf("[管理器] 停止任务 %s\n", id)
	task.Stop()

	// 2. 删除任务文件
	fmt.Printf("[管理器] 删除任务 %s 的文件\n", id)
	err := task.DeleteFiles()

	// 3. 从管理器中删除任务
	tm.lock.Lock()
	delete(tm.tasks, id)
	tm.lock.Unlock()

	fmt.Printf("[管理器] 任务 %s 已从管理器中删除\n", id)
	return true, err
}

// DeleteTask 仅从管理器中删除任务，不停止下载和删除文件
func (tm *TaskManager) DeleteTask(id string) bool {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	if task, exists := tm.tasks[id]; exists {
		// 取消文件名占用
		fileKey := tm.getFileKey(task.Output, task.FileName)
		delete(tm.fileNameMap, fileKey)

		delete(tm.tasks, id)
		return true
	}
	return false
}

// CheckFileNameExists 检查指定目录下的文件名是否已被占用
func (tm *TaskManager) CheckFileNameExists(folder, fileName string) bool {
	tm.lock.RLock()
	defer tm.lock.RUnlock()

	fileKey := tm.getFileKey(folder, fileName)
	return tm.fileNameMap[fileKey]
}

// GenerateUniqueFileName 为指定目录生成唯一的文件名
func (tm *TaskManager) GenerateUniqueFileName(folder, baseFileName string) string {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	finalFileName := baseFileName
	counter := 1

	// 检查文件名是否已被占用（包括任务管理器中的和文件系统中的）
	for {
		// 检查任务管理器中是否已占用
		fileKey := tm.getFileKey(folder, finalFileName)

		// 检查文件系统中是否已存在同名文件
		fullPath := filepath.Join(folder, finalFileName)
		fileExists := tm.fileNameMap[fileKey] || fileExistsOnDisk(fullPath)

		if !fileExists {
			break // 文件名未被占用且磁盘上不存在，可以使用
		}

		// 文件名已被占用或文件已存在，生成新文件名
		ext := filepath.Ext(baseFileName)
		baseName := strings.TrimSuffix(baseFileName, ext)
		finalFileName = fmt.Sprintf("%s_%d%s", baseName, counter, ext)
		counter++
	}

	// 标记新文件名为已占用
	fileKey := tm.getFileKey(folder, finalFileName)
	tm.fileNameMap[fileKey] = true

	return finalFileName
}

// fileExistsOnDisk 检查文件系统中是否存在指定路径的文件
func fileExistsOnDisk(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// getFileKey 生成文件唯一标识键
// 使用完整路径作为键，包括文件名和扩展名
func (tm *TaskManager) getFileKey(folder, fileName string) string {
	return filepath.Join(folder, fileName)
}

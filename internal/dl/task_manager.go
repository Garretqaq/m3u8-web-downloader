package dl

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"m3u8-go/internal/config"
)

// TaskManager 管理所有下载任务
type TaskManager struct {
	lock              sync.RWMutex
	tasks             map[string]*Downloader // 使用任务ID作为key
	fileNameMap       map[string]bool        // 记录已被占用的文件名，格式: "文件夹路径:文件名"
	downloadingSem    chan struct{}          // 用于控制同时下载的数量
	maxConcurrent     int                    // 最大同时下载数量
	downloadQueue     []*Downloader          // 等待下载的任务队列
	queueLock         sync.Mutex             // 队列锁
	downloadQueueTick *time.Ticker           // 下载队列定时器
	speedLimit        int                    // 下载速度限制，单位KB/s，0表示不限制
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
			tasks:          make(map[string]*Downloader),
			fileNameMap:    make(map[string]bool),
			downloadingSem: make(chan struct{}, 3),
			downloadQueue:  make([]*Downloader, 0),
			speedLimit:     0, // 默认不限制下载速度
		}

		// 根据配置初始化最大并发下载数量
		cfg := config.Get()
		max := cfg.MaxConcurrentDownload
		if max <= 0 {
			max = 3
		} else if max > 10 {
			max = 10
		}

		instance.maxConcurrent = max
		instance.downloadingSem = make(chan struct{}, max)

		instance.downloadQueue = make([]*Downloader, 0)
		instance.speedLimit = 0 // 初始不限速

		// 启动队列处理器
		go instance.startQueueProcessor()
		fmt.Printf("[任务管理器] 初始化完成，默认同时下载数量: %d\n", instance.maxConcurrent)
	})
	return instance
}

// UpdateMaxConcurrentDownloads 更新最大并发下载数量
func (tm *TaskManager) UpdateMaxConcurrentDownloads(max int) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	if max < 1 {
		max = 1
	} else if max > 10 {
		max = 10
	}

	if tm.maxConcurrent != max {
		tm.maxConcurrent = max
		// 重新初始化信号量
		tm.downloadingSem = make(chan struct{}, max)

		// 重新处理队列中的任务
		go tm.checkQueuedTasks()
	}
}

// UpdateDownloadSpeedLimit 更新下载速度限制
func (tm *TaskManager) UpdateDownloadSpeedLimit(limit int) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	// 如果速度限制小于0，则设置为0（不限制）
	if limit < 0 {
		limit = 0
	}

	fmt.Printf("[任务管理器] 更新下载速度限制为 %d KB/s\n", limit)
	tm.speedLimit = limit
}

// GetDownloadSpeedLimit 获取当前下载速度限制
func (tm *TaskManager) GetDownloadSpeedLimit() int {
	tm.lock.RLock()
	defer tm.lock.RUnlock()
	return tm.speedLimit
}

// GetMaxConcurrentDownloads 获取当前最大并发下载数量
func (tm *TaskManager) GetMaxConcurrentDownloads() int {
	tm.lock.RLock()
	defer tm.lock.RUnlock()
	return tm.maxConcurrent
}

// startQueueProcessor 启动队列处理器
func (tm *TaskManager) startQueueProcessor() {
	fmt.Println("[队列处理器] 启动下载队列处理器")

	// 确保之前的定时器被停止（如果存在）
	if tm.downloadQueueTick != nil {
		tm.downloadQueueTick.Stop()
	}

	// 创建新的定时器，每10秒检查一次队列
	tm.downloadQueueTick = time.NewTicker(10 * time.Second)

	// 在启动时立即检查一次队列
	go tm.checkQueuedTasks()

	// 启动定时检查
	go func() {
		for range tm.downloadQueueTick.C {
			// 捕获潜在的崩溃，确保队列处理器不会停止
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("[队列处理器] 处理队列时出现错误: %v，已恢复继续运行\n", r)
					}
				}()

				tm.checkQueuedTasks()
			}()
		}
	}()

	fmt.Println("[队列处理器] 队列处理器已启动并运行中")
}

// checkQueuedTasks 检查排队中的任务，尝试启动新的任务
func (tm *TaskManager) checkQueuedTasks() {
	tm.queueLock.Lock()
	defer tm.queueLock.Unlock()

	queueLength := len(tm.downloadQueue)
	if queueLength == 0 {
		// 没有等待的任务
		return
	}

	fmt.Printf("[队列处理] 开始处理等待队列，当前队列长度: %d\n", queueLength)

	// 检查当前可用槽位数
	availableSlots := tm.maxConcurrent - len(tm.downloadingSem)
	fmt.Printf("[队列处理] 当前可用下载槽位: %d (最大:%d, 使用中:%d)\n",
		availableSlots, tm.maxConcurrent, len(tm.downloadingSem))

	if availableSlots <= 0 {
		fmt.Println("[队列处理] 无可用下载槽位，等待中...")
		return
	}

	// 尝试启动队列中的任务
	newQueue := make([]*Downloader, 0, queueLength)
	tasksStarted := 0

	for _, task := range tm.downloadQueue {
		// 确认任务仍然处于等待状态
		if task.Status != StatusPending {
			fmt.Printf("[队列处理] 任务 %s 状态异常: %s，从队列中移除\n", task.ID, task.Status)
			continue // 跳过状态不正确的任务
		}

		// 非阻塞方式尝试获取下载槽位
		select {
		case tm.downloadingSem <- struct{}{}:
			// 成功获取槽位，启动下载
			fmt.Printf("[队列处理] 成功获取槽位，启动任务 %s\n", task.ID)
			task.Status = StatusDownloading
			task.Message = "正在下载"
			tasksStarted++

			// 异步开始下载
			go func(t *Downloader) {
				defer func() {
					// 下载完成后释放槽位
					<-tm.downloadingSem
					fmt.Printf("[队列处理] 任务 %s 完成，已释放下载槽位\n", t.ID)

					// 下载完成后检查队列，可能有等待的任务
					tm.checkQueuedTasks()
				}()

				// 确保线程数至少为1
				if t.C <= 0 {
					t.C = config.Get().DefaultThreadCount // 使用默认值
				}
				fmt.Printf("[队列处理] 任务 %s 开始下载，线程数: %d\n", t.ID, t.C)

				if err := t.Start(t.C); err != nil {
					t.Status = StatusFailed
					t.Message = "下载失败: " + err.Error()
					fmt.Printf("[队列处理] 任务 %s 下载失败: %s\n", t.ID, err)
				}
			}(task)
		default:
			// 没有可用槽位，保留在队列中
			fmt.Printf("[队列处理] 无可用槽位，任务 %s 保留在队列\n", task.ID)
			newQueue = append(newQueue, task)
		}
	}

	tm.downloadQueue = newQueue
	fmt.Printf("[队列处理] 队列处理完成，启动了 %d 个任务，剩余 %d 个任务在队列\n",
		tasksStarted, len(tm.downloadQueue))
}

// EnqueueDownload 将下载任务加入队列
func (tm *TaskManager) EnqueueDownload(task *Downloader) {
	// 先添加到任务管理器
	tm.AddTask(task)

	// 设置任务状态为等待中
	task.Status = StatusPending
	task.Message = "排队等待下载"

	// 尝试立即下载，如果槽位已满则加入队列
	select {
	case tm.downloadingSem <- struct{}{}:
		// 有可用槽位，直接开始下载
		task.Status = StatusDownloading
		task.Message = "正在下载"

		// 异步开始下载
		go func(t *Downloader) {
			defer func() {
				// 下载完成后释放槽位
				<-tm.downloadingSem
				// 下载完成后检查队列，可能有等待的任务
				tm.checkQueuedTasks()
			}()

			if err := t.Start(t.C); err != nil {
				t.Status = StatusFailed
				t.Message = "下载失败: " + err.Error()
			}
		}(task)
	default:
		// 没有可用槽位，加入等待队列
		tm.queueLock.Lock()
		tm.downloadQueue = append(tm.downloadQueue, task)
		tm.queueLock.Unlock()

		fmt.Printf("[队列] 任务 %s 加入下载队列，当前队列长度: %d\n", task.ID, len(tm.downloadQueue))
	}
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

	// 检查任务是否占用下载槽位
	// 不仅是当前正在下载的，还有刚刚从队列中被取出准备开始下载的任务
	isOccupyingSlot := task.Status == StatusDownloading || task.Status == StatusPending

	// 先解锁，因为 Stop 方法内部会获取锁
	tm.lock.Unlock()

	// 1. 停止任务下载
	fmt.Printf("[管理器] 停止任务 %s，状态: %s\n", id, task.Status)
	task.Stop()

	// 2. 删除任务文件
	fmt.Printf("[管理器] 删除任务 %s 的文件\n", id)
	err := task.DeleteFiles()

	// 3. 从管理器中删除任务
	tm.lock.Lock()
	delete(tm.tasks, id)
	tm.lock.Unlock()

	fmt.Printf("[管理器] 任务 %s 已从管理器中删除\n", id)

	// 4. 如果删除的是占用下载槽位的任务，释放下载槽位并检查队列
	// 但要注意，我们不确定这个任务是否真的占用了下载槽位，因为它可能仅仅是在队列中
	if isOccupyingSlot {
		fmt.Printf("[管理器] 任务 %s 可能占用下载槽位，尝试释放\n", id)

		// 尝试释放下载槽位，确保不会阻塞
		select {
		case <-tm.downloadingSem:
			fmt.Printf("[管理器] 成功释放任务 %s 的下载槽位\n", id)
			// 只有成功释放了槽位，才去检查队列
			go func() {
				// 延迟一小段时间，确保任务管理器状态已更新
				time.Sleep(100 * time.Millisecond)
				tm.checkQueuedTasks()
			}()
		default:
			fmt.Printf("[管理器] 任务 %s 可能并未占用下载槽位\n", id)
			// 虽然没释放成功，但还是尝试检查队列，可能有其他原因导致队列中的任务没被激活
			go tm.checkQueuedTasks()
		}
	} else {
		// 即使这个任务不是正在下载的，也检查一下队列，以防万一
		go tm.checkQueuedTasks()
	}

	return true, err
}

// DeleteTask 仅从管理器中删除任务，不停止下载和删除文件
func (tm *TaskManager) DeleteTask(id string) bool {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	if task, exists := tm.tasks[id]; exists {
		// 检查任务是否占用下载槽位
		isOccupyingSlot := task.Status == StatusDownloading || task.Status == StatusPending

		// 取消文件名占用
		fileKey := tm.getFileKey(task.Output, task.FileName)
		delete(tm.fileNameMap, fileKey)

		delete(tm.tasks, id)

		// 如果删除的是占用下载槽位的任务，释放下载槽位并检查队列
		if isOccupyingSlot {
			fmt.Printf("[管理器] DeleteTask: 任务 %s 可能占用下载槽位，尝试释放\n", id)
			// 由于有 defer tm.lock.Unlock()，需要先解锁以避免死锁
			tm.lock.Unlock()

			// 尝试释放下载槽位，确保不会阻塞
			select {
			case <-tm.downloadingSem:
				fmt.Printf("[管理器] DeleteTask: 成功释放任务 %s 的下载槽位\n", id)
				// 只有成功释放了槽位，才去检查队列
				go func() {
					// 延迟一小段时间，确保任务管理器状态已更新
					time.Sleep(100 * time.Millisecond)
					tm.checkQueuedTasks()
				}()
			default:
				fmt.Printf("[管理器] DeleteTask: 任务 %s 可能并未占用下载槽位\n", id)
				// 虽然没释放成功，但还是尝试检查队列
				go tm.checkQueuedTasks()
			}

			// 由于已手动解锁，但函数结束时还有defer解锁，需要重新加锁
			tm.lock.Lock()
		} else {
			// 即使这个任务不是正在下载的，也检查一下队列
			// 由于锁问题，需要在单独的goroutine中调用
			go func() {
				// 等待当前函数的defer解锁执行完毕
				time.Sleep(10 * time.Millisecond)
				tm.checkQueuedTasks()
			}()
		}

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

// ClearCompletedTasks 清除所有已完成的下载任务记录
func (tm *TaskManager) ClearCompletedTasks() int {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	// 计数器，记录清除的任务数量
	count := 0

	// 找出所有已完成的任务
	completedTaskIDs := make([]string, 0)
	for id, task := range tm.tasks {
		if task.Status == StatusSuccess {
			completedTaskIDs = append(completedTaskIDs, id)
			count++
		}
	}

	// 清除所有已完成的任务
	for _, id := range completedTaskIDs {
		task := tm.tasks[id]

		// 取消文件名占用
		fileKey := tm.getFileKey(task.Output, task.FileName)
		delete(tm.fileNameMap, fileKey)

		// 从任务管理器中删除任务
		delete(tm.tasks, id)

		fmt.Printf("[管理器] 已清除完成任务: %s\n", id)
	}

	return count
}

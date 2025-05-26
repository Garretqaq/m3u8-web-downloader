package dl

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"m3u8-go/internal/config"
	"m3u8-go/internal/parse"
	"m3u8-go/internal/tool"
)

const (
	tsExt            = ".ts"
	tsFolderName     = "ts"      // 这个将不再直接使用
	mergeTSFilename  = "main.ts" // 默认名称，当没有指定文件名时使用
	tsTempFileSuffix = "_tmp"
	progressWidth    = 40
	maxRetryCount    = 3 // 最大重试次数，防止无限重试

	// 任务状态常量
	StatusDownloading = "downloading" // 下载中
	StatusSuccess     = "success"     // 下载成功
	StatusFailed      = "failed"      // 下载失败
	StatusPending     = "pending"     // 等待下载
	StatusUnfinished  = "unfinished"  // 下载未完成（替代原来的stopped状态）
	StatusConverting  = "converting"  // 正在转换格式
)

type Downloader struct {
	lock     sync.Mutex
	queue    []int
	folder   string
	tsFolder string
	finish   int32
	segLen   int

	// 添加重试计数map，用于限制每个分片的重试次数
	retryCounter map[int]int

	ID                   string  // 任务ID
	Progress             int     // 下载进度 (0-100)
	Status               string  // 任务状态
	Message              string  // 状态信息
	URL                  string  // 下载链接
	Output               string  // 输出路径
	C                    int     // 线程数
	Created              int64   // 创建时间
	FileName             string  // 输出文件名
	DeleteTs             bool    // 合并完成后是否删除分片文件
	ConvertToMp4         bool    // 是否转换为MP4格式
	Speed                float64 // 下载速度（字节/秒）
	totalBytesDownloaded int64   // 已下载字节数（用于速度统计）
	TotalSize            int64   // 文件总大小（字节）

	stopChan      chan struct{} // 用于停止下载的通道
	stopped       bool          // 是否已停止
	lastBytes     int64         // 上次统计的已下载字节数
	lastSpeedTime time.Time     // 上次计算速度的时间

	result *parse.Result
}

// NewTask returns a Task instance
func NewTask(output string, url string) (*Downloader, error) {
	result, err := parse.FromURL(url)
	if err != nil {
		return nil, err
	}
	var folder string
	// If no output folder specified, use current directory
	if output == "" {
		current, err := tool.CurrentDir()
		if err != nil {
			return nil, err
		}
		folder = filepath.Join(current, output)
	} else {
		folder = output
	}
	if err := os.MkdirAll(folder, os.ModePerm); err != nil {
		return nil, fmt.Errorf("create storage folder failed: %s", err.Error())
	}

	// 生成唯一任务ID
	id := strconv.FormatInt(time.Now().UnixNano(), 10)

	// 使用任务ID创建唯一的ts文件夹名称
	tsFolderUnique := fmt.Sprintf("ts_%s", id)
	tsFolder := filepath.Join(folder, tsFolderUnique)

	if err := os.MkdirAll(tsFolder, os.ModePerm); err != nil {
		return nil, fmt.Errorf("create ts folder '[%s]' failed: %s", tsFolder, err.Error())
	}

	// 生成输出文件名
	// 从URL中提取文件名或使用默认名称
	fileName := mergeTSFilename
	urlPath := strings.TrimSpace(url)
	if strings.Contains(urlPath, "/") {
		parts := strings.Split(urlPath, "/")
		lastPart := parts[len(parts)-1]
		if strings.Contains(lastPart, ".m3u8") {
			// 替换扩展名
			fileName = strings.Replace(lastPart, ".m3u8", ".ts", 1)
		} else if lastPart != "" {
			// 如果URL最后部分不为空且不包含.m3u8，添加.ts后缀
			fileName = lastPart + ".ts"
		}
	}

	// 使用任务管理器生成唯一文件名
	taskManager := GetTaskManager()
	finalFileName := taskManager.GenerateUniqueFileName(folder, fileName)

	// 设置默认线程数，统一来自配置
	defaultThreadCount := config.Get().DefaultThreadCount

	d := &Downloader{
		folder:               folder,
		tsFolder:             tsFolder,
		result:               result,
		ID:                   id,
		Progress:             0,
		Status:               StatusPending,
		Message:              "等待下载",
		URL:                  url,
		Output:               output,
		FileName:             finalFileName,
		Created:              time.Now().Unix(),
		stopChan:             make(chan struct{}),
		stopped:              false,
		DeleteTs:             false, // 默认不删除分片文件
		ConvertToMp4:         false, // 默认不转换为MP4
		Speed:                0,     // 初始下载速度为0
		totalBytesDownloaded: 0,
		lastBytes:            0,
		lastSpeedTime:        time.Now(),
		C:                    defaultThreadCount, // 设置默认线程数
		retryCounter:         make(map[int]int),  // 初始化重试计数器
		TotalSize:            0,                  // 初始化文件总大小
	}
	d.segLen = len(result.M3u8.Segments)
	d.queue = genSlice(d.segLen)
	return d, nil
}

// Start runs downloader
func (d *Downloader) Start(concurrency int) error {
	d.C = concurrency
	d.Status = StatusDownloading
	d.Message = "正在下载"
	d.stopped = false                  // 重置停止标志
	d.stopChan = make(chan struct{})   // 重新创建停止通道
	d.retryCounter = make(map[int]int) // 重置重试计数器

	// 获取限速设置并记录日志
	taskManager := GetTaskManager()
	speedLimit := taskManager.GetDownloadSpeedLimit()
	if speedLimit > 0 {
		tool.Info("[任务 %s] 启动下载 - 线程数: %d, 全局限速: %d KB/s",
			d.ID, concurrency, speedLimit)
	} else {
		tool.Info("[任务 %s] 启动下载 - 线程数: %d, 不限速", d.ID, concurrency)
	}

	var wg sync.WaitGroup
	// struct{} zero size
	limitChan := make(chan struct{}, concurrency)

	// 停止下载任务的标志
	stopFlag := false

	// 监听停止信号
	go func() {
		<-d.stopChan
		d.lock.Lock()
		// 只标记队列清空和停止标志，但不修改任务状态
		d.queue = nil // 清空队列
		d.stopped = true
		stopFlag = true
		tool.Info("[task %s] 收到停止信号，仅中断下载流程", d.ID)
		d.lock.Unlock()
	}()

	// 设置清理函数，确保在函数退出时总是能等待所有协程完成
	defer func() {
		// 如果函数退出但没有正常结束，保持当前状态，不设置为"已停止"
		d.lock.Lock()
		if d.Status != StatusSuccess && d.Status != StatusFailed {
			// 如果没有成功或失败，保持当前状态，但记录中断
			if d.Message == "" {
				d.Message = "下载未完成"
			}
		}
		d.lock.Unlock()

		// 等待所有工作协程完成，添加超时保护
		waitChan := make(chan struct{})
		go func() {
			wg.Wait()
			close(waitChan)
		}()

		// 等待协程完成或超时（最多等待5秒）
		select {
		case <-waitChan:
			tool.Info("[task %s] 所有下载协程已完成", d.ID)
		case <-time.After(5 * time.Second):
			tool.Warning("[task %s] 等待协程超时，可能有泄漏的协程", d.ID)
		}

		// 确保stopChan被关闭，但不修改任务状态
		d.lock.Lock()
		currentStatus := d.Status
		select {
		case <-d.stopChan:
			// 已经关闭，不需要再次关闭
		default:
			close(d.stopChan)
		}

		// 标记为已停止内部状态，但不修改对外显示的状态
		if currentStatus != StatusSuccess && currentStatus != StatusFailed {
			d.stopped = true
		} else if currentStatus == StatusSuccess {
			// 确保成功状态下消息正确
			if d.ConvertToMp4 {
				d.Message = fmt.Sprintf("下载完成并合并为MP4: %s", d.FileName)
			} else {
				d.Message = fmt.Sprintf("下载完成: %s", d.FileName)
			}
		}
		d.lock.Unlock()
	}()

	// 添加安全检查，防止段索引越界
	if d.segLen <= 0 || d.result == nil || d.result.M3u8 == nil || len(d.result.M3u8.Segments) == 0 {
		d.Status = StatusFailed
		d.Message = "无效的M3U8数据，没有可下载的分片"
		return fmt.Errorf("invalid m3u8 data: no segments to download")
	}

	// 主下载循环
downloadLoop:
	for !stopFlag {
		tsIdx, end, err := d.next()
		if err != nil {
			if end {
				break downloadLoop
			}

			// 添加短暂延迟，避免CPU满负荷循环
			time.Sleep(20 * time.Millisecond)
			continue
		}

		// 安全检查
		if tsIdx < 0 || tsIdx >= d.segLen {
			tool.Error("[error] Invalid segment index: %d (range: 0-%d)", tsIdx, d.segLen-1)
			continue
		}

		wg.Add(1)
		go func(idx int) {
			defer func() {
				wg.Done()
				// 捕获协程中的panic
				if r := recover(); r != nil {
					tool.Error("[panic] 下载协程异常退出: %v", r)
				}
			}()

			// 检查是否已停止
			if d.stopped {
				<-limitChan
				return
			}

			if err := d.download(idx); err != nil {
				// Back into the queue, retry request
				tool.Warning("[failed] %s", err.Error())
				if !d.stopped { // 只有在没有停止的情况下才重试
					if err := d.back(idx); err != nil {
						tool.Error("%s", err.Error())
					}
				}
			}
			<-limitChan
		}(tsIdx)
		limitChan <- struct{}{}

		// 添加周期性的进度汇报和健康检查
		if tsIdx%50 == 0 {
			tool.Info("[progress] 已分发 %d/%d 个分片任务，完成：%d，进度：%d%%",
				tsIdx, d.segLen, atomic.LoadInt32(&d.finish), d.Progress)

			// 检查是否有太多失败，提前终止可能无法完成的任务
			if atomic.LoadInt32(&d.finish) < int32(float32(tsIdx)*0.3) && tsIdx > 100 {
				tool.Warning("[warning] 成功率过低，可能遇到严重问题")
			}
		}
	}

	tool.Info("[task %s] 等待所有下载协程完成", d.ID)

	// 等待所有协程完成
	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()

	// 等待所有协程完成或超时
	select {
	case <-waitDone:
		tool.Info("[task %s] 所有分片下载协程已完成", d.ID)
	case <-time.After(30 * time.Second):
		tool.Warning("[task %s] 等待下载协程超时，继续后续处理", d.ID)
	}

	// 如果下载已停止，直接返回
	if d.stopped {
		tool.Info("[task %s] 任务已停止，跳过合并步骤", d.ID)
		return nil
	}

	// 检查进度，如果完成的数量太少，可以选择不进行合并
	finishedCount := atomic.LoadInt32(&d.finish)
	if finishedCount < int32(float32(d.segLen)*0.5) {
		d.Status = StatusFailed
		d.Message = fmt.Sprintf("下载失败: 只完成了 %d/%d 个分片，无法合并", finishedCount, d.segLen)
		return fmt.Errorf("too few segments downloaded (%d of %d)", finishedCount, d.segLen)
	}

	// 重要修改：在合并前释放下载槽位
	// 通知任务管理器释放当前任务的下载槽位，这样合并过程不会占用下载限制
	taskManager = GetTaskManager()
	taskManager.ReleaseDownloadSlot(d.ID)
	tool.Info("[task %s] 下载阶段完成，释放下载槽位准备进行合并", d.ID)

	// 尝试合并，如果合并失败则设置相应状态
	if err := d.merge(); err != nil {
		d.Status = StatusFailed
		d.Message = "合并失败: " + err.Error()
		return err
	}

	// 确保下载成功时状态一致，修复多文件合并后显示已停止的bug
	d.lock.Lock()
	prevStatus := d.Status
	d.Status = StatusSuccess // 确保设置状态为成功
	d.Message = "下载完成"       // 恢复消息设置
	d.Progress = 100
	d.stopped = false // 重置停止标志，确保不会被误标记为已停止
	d.lock.Unlock()

	// 获取合并后文件的实际大小并更新TotalSize字段
	filePath := filepath.Join(d.folder, d.FileName)
	if fileInfo, err := os.Stat(filePath); err == nil {
		d.TotalSize = fileInfo.Size()
		tool.Info("[info] 更新文件大小: %s (%d 字节)", d.FileName, d.TotalSize)
	} else {
		tool.Warning("[warning] 无法获取文件大小: %s, 错误: %s", filePath, err.Error())
	}

	// 添加状态转换日志，便于调试
	tool.Info("[任务 %s] 状态已从 %s 更新为 %s", d.ID, prevStatus, StatusSuccess)

	return nil
}

// Stop 停止下载任务
func (d *Downloader) Stop() {
	d.lock.Lock()
	defer d.lock.Unlock()

	// 不对已成功或失败的任务执行停止操作
	if d.Status == StatusSuccess || d.Status == StatusFailed {
		return
	}

	// 只标记内部停止状态并关闭通道，不修改任务状态
	if !d.stopped {
		// 重要：需要确保 stopChan 只关闭一次
		select {
		case <-d.stopChan:
			// 已经关闭，不需要再次关闭
		default:
			close(d.stopChan)
			d.stopped = true
			tool.Info("[task %s] 下载过程已中断", d.ID)
		}
	}
}

// DeleteFiles 删除任务相关文件
func (d *Downloader) DeleteFiles() error {
	// 如果任务正在下载或转换，先终止下载过程
	d.lock.Lock()

	// 标记为已停止内部状态，但不修改对外状态
	d.stopped = true

	// 关闭停止通道
	select {
	case <-d.stopChan:
		// 已经关闭
	default:
		close(d.stopChan)
	}

	d.lock.Unlock()

	var errs []string

	// 1. 删除TS文件夹（如果存在）
	tsFolder := d.tsFolder
	if _, err := os.Stat(tsFolder); !os.IsNotExist(err) {
		if err := os.RemoveAll(tsFolder); err != nil {
			errs = append(errs, fmt.Sprintf("删除TS临时文件夹失败: %s", err.Error()))
		}
	}

	// 2. 删除合并后的输出文件（如果存在）
	tsFileName := d.FileName
	tsFilePath := filepath.Join(d.folder, tsFileName)
	if _, err := os.Stat(tsFilePath); !os.IsNotExist(err) {
		if err := os.Remove(tsFilePath); err != nil {
			errs = append(errs, fmt.Sprintf("删除TS输出文件失败: %s", err.Error()))
		}
	}

	// 3. 如果有MP4文件，也需要删除
	if d.ConvertToMp4 {
		mp4FileName := strings.TrimSuffix(tsFileName, filepath.Ext(tsFileName)) + ".mp4"
		mp4FilePath := filepath.Join(d.folder, mp4FileName)
		if _, err := os.Stat(mp4FilePath); !os.IsNotExist(err) {
			if err := os.Remove(mp4FilePath); err != nil {
				errs = append(errs, fmt.Sprintf("删除MP4输出文件失败: %s", err.Error()))
			}
		}
	}

	// 如果有错误，返回组合的错误信息
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	return nil
}

// Resume 继续下载任务
func (d *Downloader) Resume() bool {
	d.lock.Lock()
	// 检查是否已被标记为内部停止状态
	if !d.stopped {
		d.lock.Unlock()
		return false
	}

	// 获取任务管理器
	taskManager := GetTaskManager()

	// 先恢复状态为等待中
	d.Status = StatusPending
	d.Message = "排队等待下载"
	d.stopped = false
	d.lock.Unlock()

	// 使用任务队列管理机制重新启动任务
	// 先把任务从管理器移除，因为EnqueueDownload会重新添加任务
	taskManager.DeleteTask(d.ID)

	// 通过队列机制重新启动任务
	taskManager.EnqueueDownload(d)

	tool.Info("[task %s] 任务已恢复，通过队列机制重新启动", d.ID)
	return true
}

// 修改 download 方法，添加检查暂停状态的逻辑
func (d *Downloader) download(segIndex int) error {
	// 首先检查是否已停止
	if d.stopped {
		return fmt.Errorf("task stopped")
	}

	tsFilename := tsFilename(segIndex)
	tsUrl := d.tsURL(segIndex)

	// 获取任务管理器中设置的下载速度限制
	taskManager := GetTaskManager()
	speedLimit := taskManager.GetDownloadSpeedLimit()

	// 使用速度限制选项调用Get
	var b io.ReadCloser
	var e error

	// 记录限速日志（只对部分分片记录，避免大量日志）
	if segIndex == 0 || segIndex%50 == 0 {
		if speedLimit > 0 {
			tool.Info("[限速] 任务 %s：全局限速 %d KB/s，线程数 %d，实际每线程限速约 %.1f KB/s",
				d.ID, speedLimit, d.C, float64(speedLimit)/float64(d.C))
		} else {
			tool.Info("[限速] 任务 %s：不限速", d.ID)
		}
	}

	// 使用全局限速的 Get 方法，不再需要传入限速参数
	b, e = tool.Get(tsUrl)

	if e != nil {
		return fmt.Errorf("request %s, %s", tsUrl, e.Error())
	}
	//noinspection GoUnhandledErrorResult
	defer b.Close()

	// 再次检查是否已停止
	if d.stopped {
		return fmt.Errorf("task stopped")
	}

	fPath := filepath.Join(d.tsFolder, tsFilename)
	fTemp := fPath + tsTempFileSuffix
	f, err := os.Create(fTemp)
	if err != nil {
		return fmt.Errorf("create file: %s, %s", tsFilename, err.Error())
	}
	// 将读取与写入改为流式 + 大缓冲区，减少内存与 flush 次数
	writer := bufio.NewWriterSize(f, 256*1024) // 256KB 缓冲

	rawBytes, err := io.ReadAll(b)
	if err != nil {
		f.Close() // 确保文件关闭
		return fmt.Errorf("read bytes: %s, %s", tsUrl, err.Error())
	}

	// 再次检查是否已停止
	if d.stopped {
		f.Close() // 确保文件关闭
		return fmt.Errorf("task stopped")
	}

	sf := d.result.M3u8.Segments[segIndex]
	if sf == nil {
		f.Close() // 确保文件关闭
		return fmt.Errorf("invalid segment index: %d", segIndex)
	}

	key, ok := d.result.Keys[sf.KeyIndex]
	if ok && key != "" {
		rawBytes, err = tool.AES128Decrypt(rawBytes, []byte(key),
			[]byte(d.result.M3u8.Keys[sf.KeyIndex].IV))
		if err != nil {
			f.Close() // 确保文件关闭
			return fmt.Errorf("decryt: %s, %s", tsUrl, err.Error())
		}
	}

	// 清理前同步至 TS 包首字节 0x47
	syncByte := uint8(71)
	idx := bytes.IndexByte(rawBytes, syncByte)
	if idx >= 0 {
		rawBytes = rawBytes[idx:]
	}

	if _, err := writer.Write(rawBytes); err != nil {
		f.Close() // 确保文件关闭
		return fmt.Errorf("write to %s: %s", fTemp, err.Error())
	}

	if err := writer.Flush(); err != nil {
		f.Close() // 确保文件关闭
		return fmt.Errorf("flush writer: %s", err.Error())
	}

	_ = f.Close()
	if err = os.Rename(fTemp, fPath); err != nil {
		return err
	}

	// 最后一次检查是否已停止，如果停止了就不更新进度
	if d.stopped {
		return nil
	}

	d.lock.Lock()
	// 累计已下载字节数
	atomic.AddInt64(&d.totalBytesDownloaded, int64(len(rawBytes)))

	// 计算下载速度（每秒更新一次）
	currentTime := time.Now()
	timeDiff := currentTime.Sub(d.lastSpeedTime).Seconds()
	if timeDiff >= 1.0 { // 每秒计算一次速度
		totalBytes := atomic.LoadInt64(&d.totalBytesDownloaded)
		bytesDiff := totalBytes - d.lastBytes

		// 计算速度（字节/秒）
		if bytesDiff > 0 && timeDiff > 0 {
			d.Speed = float64(bytesDiff) / timeDiff
		} else {
			// 如果没有新数据，速度可能降低但不会变为0
			d.Speed = d.Speed * 0.7 // 衰减因子
		}

		d.lastBytes = totalBytes
		d.lastSpeedTime = currentTime
	}
	d.lock.Unlock()

	// Maybe it will be safer in this way...
	atomic.AddInt32(&d.finish, 1)

	// 更新进度
	progress := int(float32(d.finish) / float32(d.segLen) * 100)
	d.Progress = progress
	d.Message = fmt.Sprintf("已下载 %d%%", progress)

	// 计算文件大小并更新总大小
	d.lock.Lock()
	d.TotalSize += int64(len(rawBytes))
	d.lock.Unlock()

	return nil
}

func (d *Downloader) next() (segIndex int, end bool, err error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	// 如果任务已停止，直接返回错误
	if d.stopped {
		err = fmt.Errorf("task stopped")
		end = true
		return
	}

	if len(d.queue) == 0 {
		err = fmt.Errorf("queue empty")
		if d.finish == int32(d.segLen) {
			end = true
			return
		}
		// Some segment indexes are still running.
		end = false
		return
	}
	segIndex = d.queue[0]
	d.queue = d.queue[1:]
	return
}

func (d *Downloader) back(segIndex int) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	// 如果任务已停止，不再将失败的分片放回队列
	if d.stopped {
		return fmt.Errorf("task stopped, segment %d not added back to queue", segIndex)
	}

	if sf := d.result.M3u8.Segments[segIndex]; sf == nil {
		return fmt.Errorf("invalid segment index: %d", segIndex)
	}

	// 检查重试次数，超过最大重试次数则不再加入队列
	d.retryCounter[segIndex]++
	if d.retryCounter[segIndex] > maxRetryCount {
		tool.Warning("[warning] segment %d exceeded max retry count (%d), skipping",
			segIndex, maxRetryCount)
		// 不将该分片加回队列，视为下载失败但继续其他分片的下载
		return nil
	}

	d.queue = append(d.queue, segIndex)
	return nil
}

func (d *Downloader) merge() error {
	// 首先检查是否已停止
	if d.stopped {
		return fmt.Errorf("task stopped, merging aborted")
	}

	// In fact, the number of downloaded segments should be equal to number of m3u8 segments
	missingCount := 0
	var missingSegments []int
	for idx := 0; idx < d.segLen; idx++ {
		tsFilename := tsFilename(idx)
		f := filepath.Join(d.tsFolder, tsFilename)
		if _, err := os.Stat(f); err != nil {
			missingCount++
			missingSegments = append(missingSegments, idx)
		}
	}

	// 如果缺失文件太多，合并可能没有意义
	if missingCount > d.segLen/2 {
		return fmt.Errorf("too many missing segments (%d of %d), aborting merge",
			missingCount, d.segLen)
	}

	if missingCount > 0 {
		tool.Warning("[warning] %d files missing. Segments: %v", missingCount, missingSegments)
	}

	// 准备所有存在的TS文件名
	tsFiles := make([]string, 0, d.segLen-missingCount)
	for segIndex := 0; segIndex < d.segLen; segIndex++ {
		tsFilename := tsFilename(segIndex)
		tsPath := filepath.Join(d.tsFolder, tsFilename)
		// 只添加存在的文件
		if _, err := os.Stat(tsPath); err == nil {
			tsFiles = append(tsFiles, tsFilename)
		}
	}

	// 如果没有文件可以合并，直接返回错误
	if len(tsFiles) == 0 {
		return fmt.Errorf("no files to merge")
	}

	// 根据是否需要转换为MP4决定输出文件路径和扩展名
	outputExt := ".ts"
	if d.ConvertToMp4 {
		outputExt = ".mp4"
	}

	// 确保文件名有正确的扩展名
	baseFileName := strings.TrimSuffix(d.FileName, filepath.Ext(d.FileName))
	outputFileName := baseFileName + outputExt
	outputPath := filepath.Join(d.folder, outputFileName)

	// 使用任务管理器生成唯一文件名，避免覆盖已有文件
	taskManager := GetTaskManager()

	// 先从任务管理器中删除原文件名占用
	taskManager.DeleteTask(d.ID)

	// 生成新的唯一文件名
	uniqueFileName := taskManager.GenerateUniqueFileName(d.folder, outputFileName)
	outputPath = filepath.Join(d.folder, uniqueFileName)

	// 重新添加任务到管理器，更新文件名占用
	d.FileName = uniqueFileName
	taskManager.AddTask(d)

	// 根据是否需要转换为MP4选择不同的合并方法
	if d.ConvertToMp4 {
		// 直接将TS分片合并为MP4
		d.Status = StatusConverting
		d.Message = "正在合并为MP4格式..."

		tool.Info("[info] 开始直接合并为MP4: %s", outputPath)

		err := tool.MergeTsToMp4(d.tsFolder, tsFiles, outputPath)
		if err != nil {
			errMsg := fmt.Sprintf("合并MP4失败: %s", err.Error())
			d.Message = errMsg
			tool.Error(errMsg)
			return fmt.Errorf(errMsg)
		}

		tool.Info("[info] MP4合并成功: %s", outputPath)

		// 更新任务完成消息
		d.Message = fmt.Sprintf("下载完成并合并为MP4: %s", d.FileName)

		// 确保设置状态为成功，修复格式转换完成后显示"已停止"的bug
		d.lock.Lock()
		prevStatus := d.Status
		d.Status = StatusSuccess
		d.stopped = false // 重置停止标志，确保不会被误标记为已停止
		d.lock.Unlock()

		// 添加状态转换日志，便于调试
		tool.Info("[任务 %s] MP4转换完成：状态从 %s 更新为 %s", d.ID, prevStatus, StatusSuccess)
	} else {
		// 优化的TS文件合并方法，使用分块处理
		tool.Info("[info] 开始合并TS文件: %s", outputPath)

		// 创建输出文件
		mFile, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("create main TS file failed：%s", err.Error())
		}
		defer mFile.Close()

		// 使用缓冲写入器提高性能
		writer := bufio.NewWriterSize(mFile, 4*1024*1024) // 4MB缓冲区

		// 分块读取和写入，减少内存占用
		const bufferSize = 8 * 1024 * 1024 // 8MB的缓冲区
		buffer := make([]byte, bufferSize)

		mergedCount := 0
		totalSegments := len(tsFiles)

		for _, tsFilename := range tsFiles {
			// 中途检查是否已停止
			if d.stopped {
				return fmt.Errorf("task stopped during merging")
			}

			tsFilePath := filepath.Join(d.tsFolder, tsFilename)

			// 打开TS分片文件
			tsFile, err := os.Open(tsFilePath)
			if err != nil {
				tool.Warning("[warning] 无法打开文件 %s: %s", tsFilePath, err.Error())
				continue
			}

			// 使用缓冲读取器提高性能
			reader := bufio.NewReaderSize(tsFile, 1024*1024) // 1MB缓冲区

			// 分块读取和写入
			for {
				n, err := reader.Read(buffer)
				if n > 0 {
					// 写入读取到的数据块
					if _, writeErr := writer.Write(buffer[:n]); writeErr != nil {
						tsFile.Close()
						return fmt.Errorf("写入合并文件失败: %s", writeErr.Error())
					}
				}

				// 检查是否读取完毕
				if err != nil {
					if err != io.EOF {
						tsFile.Close()
						tool.Warning("[warning] 读取文件 %s 时出错: %s", tsFilePath, err.Error())
					}
					break
				}
			}

			// 关闭当前TS文件
			tsFile.Close()

			// 更新进度
			mergedCount++
			progress := int(float32(mergedCount) / float32(totalSegments) * 100)
			d.Message = fmt.Sprintf("合并中 %d%%", progress)
			tool.DrawProgressBar("merge", float32(mergedCount)/float32(totalSegments), progressWidth)
		}

		// 确保所有数据都写入到文件
		if err := writer.Flush(); err != nil {
			return fmt.Errorf("flush file error: %s", err.Error())
		}

		if mergedCount != totalSegments {
			tool.Warning("[warning] \n%d files merge failed", totalSegments-mergedCount)
		}

		// 更新任务完成消息
		d.Message = fmt.Sprintf("下载完成: %s", outputFileName)
		tool.Info("[info] TS合并成功: %s", outputPath)
	}

	tool.Info("\n[output] %s", outputPath)

	// 根据DeleteTs字段决定是否删除分片文件夹
	if d.DeleteTs {
		tool.Info("[info] 删除TS分片文件夹: %s", d.tsFolder)
		_ = os.RemoveAll(d.tsFolder)
	}

	// 确保任务状态被设置为成功，解决多文件下载后合并完成但显示为"已停止"的问题
	d.lock.Lock()
	prevStatus := d.Status
	d.Status = StatusSuccess // 确保设置状态为成功
	d.stopped = false        // 重置停止标志，确保不会被误标记为已停止
	d.Progress = 100

	// 获取合并后文件的实际大小并更新TotalSize字段
	if fileInfo, err := os.Stat(outputPath); err == nil {
		d.TotalSize = fileInfo.Size()
		tool.Info("[info] 更新文件大小: %s (%d 字节)", d.FileName, d.TotalSize)
	} else {
		tool.Warning("[warning] 无法获取文件大小: %s, 错误: %s", outputPath, err.Error())
	}

	d.lock.Unlock()

	// 添加状态转换日志，便于调试
	tool.Info("[任务 %s] 状态已从 %s 更新为 %s", d.ID, prevStatus, StatusSuccess)

	return nil
}

func (d *Downloader) tsURL(segIndex int) string {
	seg := d.result.M3u8.Segments[segIndex]
	return tool.ResolveURL(d.result.URL, seg.URI)
}

func tsFilename(ts int) string {
	return strconv.Itoa(ts) + tsExt
}

func genSlice(len int) []int {
	s := make([]int, 0)
	for i := 0; i < len; i++ {
		s = append(s, i)
	}
	return s
}

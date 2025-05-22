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

	// 任务状态常量
	StatusDownloading = "downloading" // 下载中
	StatusSuccess     = "success"     // 下载成功
	StatusFailed      = "failed"      // 下载失败
	StatusPending     = "pending"     // 等待下载
	StatusStopped     = "stopped"     // 已停止
	StatusConverting  = "converting"  // 正在转换格式
)

type Downloader struct {
	lock     sync.Mutex
	queue    []int
	folder   string
	tsFolder string
	finish   int32
	segLen   int

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
	d.stopped = false                // 重置停止标志
	d.stopChan = make(chan struct{}) // 重新创建停止通道

	var wg sync.WaitGroup
	// struct{} zero size
	limitChan := make(chan struct{}, concurrency)

	// 停止下载任务的标志
	stopFlag := false

	// 监听停止信号
	go func() {
		<-d.stopChan
		d.lock.Lock()
		d.queue = nil // 清空队列
		d.Status = StatusStopped
		d.Message = "下载已停止"
		d.stopped = true
		d.lock.Unlock()
	}()

	defer func() {
		// 如果函数退出但没有正常结束，标记为已停止
		if d.Status != StatusSuccess && d.Status != StatusFailed {
			d.Status = StatusStopped
		}
	}()

	for {
		// 检查任务是否已停止
		if stopFlag {
			break
		}

		tsIdx, end, err := d.next()
		if err != nil {
			if end {
				break
			}
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// 检查是否已停止
			if d.stopped {
				<-limitChan
				return
			}

			if err := d.download(idx); err != nil {
				// Back into the queue, retry request
				fmt.Printf("[failed] %s\n", err.Error())
				if !d.stopped { // 只有在没有停止的情况下才重试
					if err := d.back(idx); err != nil {
						fmt.Printf("%s", err.Error())
					}
				}
			}
			<-limitChan
		}(tsIdx)
		limitChan <- struct{}{}
	}

	fmt.Printf("[task %s] 等待所有下载协程完成\n", d.ID)
	wg.Wait()

	// 如果下载已停止，直接返回
	if d.stopped {
		fmt.Printf("[task %s] 任务已停止，跳过合并步骤\n", d.ID)
		return nil
	}

	if err := d.merge(); err != nil {
		d.Status = StatusFailed
		d.Message = "合并失败: " + err.Error()
		return err
	}

	d.Status = StatusSuccess
	d.Message = "下载完成"
	d.Progress = 100
	return nil
}

// Stop 停止下载任务
func (d *Downloader) Stop() {
	d.lock.Lock()
	defer d.lock.Unlock()

	// 只有当任务未停止且处于下载中或等待状态时才停止
	if !d.stopped && (d.Status == StatusDownloading || d.Status == StatusPending) {
		// 重要：需要确保 stopChan 只关闭一次
		select {
		case <-d.stopChan:
			// 已经关闭，不需要再次关闭
		default:
			close(d.stopChan)
			d.stopped = true
			d.Status = StatusStopped
			d.Message = "下载已停止"
			fmt.Printf("[task %s] 下载已停止\n", d.ID)
		}
	}
}

// DeleteFiles 删除任务相关文件
func (d *Downloader) DeleteFiles() error {
	// 如果任务正在下载，先停止下载
	if d.Status == StatusDownloading || d.Status == StatusConverting {
		d.Stop()
	}

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
	if d.Status != StatusStopped {
		d.lock.Unlock()
		return false
	}

	// 获取任务管理器
	taskManager := GetTaskManager()

	// 先恢复状态为等待中
	d.Status = StatusPending
	d.Message = "排队等待下载"
	d.lock.Unlock()

	// 使用任务队列管理机制重新启动任务
	// 先把任务从管理器移除，因为EnqueueDownload会重新添加任务
	taskManager.DeleteTask(d.ID)

	// 通过队列机制重新启动任务
	taskManager.EnqueueDownload(d)

	fmt.Printf("[task %s] 任务已恢复，通过队列机制重新启动\n", d.ID)
	return true
}

// 修改 download 方法，添加检查暂停状态的逻辑
func (d *Downloader) download(segIndex int) error {
	tsFilename := tsFilename(segIndex)
	tsUrl := d.tsURL(segIndex)
	b, e := tool.Get(tsUrl)
	if e != nil {
		return fmt.Errorf("request %s, %s", tsUrl, e.Error())
	}
	//noinspection GoUnhandledErrorResult
	defer b.Close()
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
		return fmt.Errorf("read bytes: %s, %s", tsUrl, err.Error())
	}

	sf := d.result.M3u8.Segments[segIndex]
	if sf == nil {
		return fmt.Errorf("invalid segment index: %d", segIndex)
	}

	key, ok := d.result.Keys[sf.KeyIndex]
	if ok && key != "" {
		rawBytes, err = tool.AES128Decrypt(rawBytes, []byte(key),
			[]byte(d.result.M3u8.Keys[sf.KeyIndex].IV))
		if err != nil {
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
		return fmt.Errorf("write to %s: %s", fTemp, err.Error())
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush writer: %s", err.Error())
	}

	_ = f.Close()
	if err = os.Rename(fTemp, fPath); err != nil {
		return err
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

	return nil
}

func (d *Downloader) next() (segIndex int, end bool, err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
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
	if sf := d.result.M3u8.Segments[segIndex]; sf == nil {
		return fmt.Errorf("invalid segment index: %d", segIndex)
	}
	d.queue = append(d.queue, segIndex)
	return nil
}

func (d *Downloader) merge() error {
	// In fact, the number of downloaded segments should be equal to number of m3u8 segments
	missingCount := 0
	for idx := 0; idx < d.segLen; idx++ {
		tsFilename := tsFilename(idx)
		f := filepath.Join(d.tsFolder, tsFilename)
		if _, err := os.Stat(f); err != nil {
			missingCount++
		}
	}
	if missingCount > 0 {
		fmt.Printf("[warning] %d files missing\n", missingCount)
	}

	// 准备所有TS文件名
	tsFiles := make([]string, 0, d.segLen)
	for segIndex := 0; segIndex < d.segLen; segIndex++ {
		tsFilename := tsFilename(segIndex)
		tsFiles = append(tsFiles, tsFilename)
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

		fmt.Printf("[info] 开始直接合并为MP4: %s\n", outputPath)

		err := tool.MergeTsToMp4(d.tsFolder, tsFiles, outputPath)
		if err != nil {
			errMsg := fmt.Sprintf("合并MP4失败: %s", err.Error())
			d.Message = errMsg
			fmt.Println(errMsg)
			return fmt.Errorf(errMsg)
		}

		fmt.Printf("[info] MP4合并成功: %s\n", outputPath)

		// 更新任务完成消息
		d.Message = fmt.Sprintf("下载完成并合并为MP4: %s", d.FileName)
	} else {
		// 优化的TS文件合并方法，使用分块处理
		d.Message = "正在合并TS文件..."
		fmt.Printf("[info] 开始合并TS文件: %s\n", outputPath)

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
		totalSegments := d.segLen

		for segIndex := 0; segIndex < totalSegments; segIndex++ {
			tsFilename := tsFilename(segIndex)
			tsFilePath := filepath.Join(d.tsFolder, tsFilename)

			// 打开TS分片文件
			tsFile, err := os.Open(tsFilePath)
			if err != nil {
				fmt.Printf("[warning] 无法打开文件 %s: %s\n", tsFilePath, err.Error())
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
			return fmt.Errorf("刷新缓冲区失败: %s", err.Error())
		}

		if mergedCount != totalSegments {
			fmt.Printf("[warning] \n%d files merge failed\n", totalSegments-mergedCount)
		}

		// 更新任务完成消息
		d.Message = fmt.Sprintf("下载完成: %s", outputFileName)
		fmt.Printf("[info] TS合并成功: %s\n", outputPath)
	}

	fmt.Printf("\n[output] %s\n", outputPath)

	// 根据DeleteTs字段决定是否删除分片文件夹
	if d.DeleteTs {
		fmt.Printf("[info] 删除TS分片文件夹: %s\n", d.tsFolder)
		_ = os.RemoveAll(d.tsFolder)
	}

	// 设置任务状态为成功
	d.Status = StatusSuccess
	d.Progress = 100

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
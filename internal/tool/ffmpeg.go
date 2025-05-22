package tool

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

// CopyKwArgs 复制 ffmpeg.KwArgs 参数
func CopyKwArgs(k ffmpeg.KwArgs) ffmpeg.KwArgs {
	newArgs := make(ffmpeg.KwArgs)
	for key, value := range k {
		newArgs[key] = value
	}
	return newArgs
}

// 预定义 FFmpeg 参数配置
var (
	// 基础内存限制参数
	baseOptions = ffmpeg.KwArgs{
		"max_muxing_queue_size": 4096,             // 增大队列大小
		"threads":               runtime.NumCPU(), // 使用所有可用CPU核心
	}

	// MP4基础参数
	mp4BaseOptions = ffmpeg.KwArgs{
		"c:v":      "copy",      // 复制视频流，不重新编码
		"c:a":      "aac",       // 使用AAC编码音频
		"movflags": "faststart", // 优化MP4文件结构，方便网络播放
	}

	// 合并后的完整参数配置
	mp4OutputOptions, m3u8ToMp4Options ffmpeg.KwArgs

	// M3U8输入参数配置
	m3u8InputOptions = ffmpeg.KwArgs{
		"protocol_whitelist":  "file,http,https,tcp,tls",
		"allowed_extensions":  "m3u8,ts",
		"reconnect":           1, // 断线重连
		"reconnect_at_eof":    1, // 文件结束时重连
		"reconnect_streamed":  1, // 流媒体重连
		"reconnect_delay_max": 5, // 最大重连延迟(秒)
	}

	// concat 参数
	concatOptions = ffmpeg.KwArgs{
		"f":    "concat",
		"safe": "0",
	}

	// 批处理中间文件参数
	batchOutputOptions = ffmpeg.KwArgs{
		"c": "copy", // 直接复制流，不做转码
	}

	// 默认超时设置
	defaultTimeout = 30 * time.Minute
)

// init 初始化函数，用于设置 ffmpeg 的路径和合并参数配置
func init() {
	// 合并参数配置
	mp4OutputOptions = CopyKwArgs(mp4BaseOptions)
	for k, v := range baseOptions {
		mp4OutputOptions[k] = v
	}

	// M3U8转MP4特殊参数
	m3u8ToMp4Options = CopyKwArgs(mp4OutputOptions)
	m3u8ToMp4Options["bsf:a"] = "aac_adtstoasc" // M3U8转MP4需要的特殊参数

	// 检测系统是否支持硬件加速
	// 注意：硬件加速需要在实际使用前验证可用性
	if runtime.GOOS == "darwin" {
		// macOS 使用 VideoToolbox
		m3u8ToMp4Options["hwaccel"] = "videotoolbox"
	} else if runtime.GOOS == "linux" {
		// Linux 尝试使用 VAAPI
		m3u8ToMp4Options["hwaccel"] = "vaapi"
	} else if runtime.GOOS == "windows" {
		// Windows 尝试使用 DXVA2
		m3u8ToMp4Options["hwaccel"] = "dxva2"
	}
}

// ConvertToMp4 将TS文件转换为MP4
// 使用 ffmpeg-go 库进行格式转换
func ConvertToMp4(inputPath, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	return ConvertToMp4WithContext(ctx, inputPath, outputPath)
}

// ConvertToMp4WithContext 带上下文的TS转MP4
func ConvertToMp4WithContext(ctx context.Context, inputPath, outputPath string) error {
	// 使用带上下文的ffmpeg命令
	cmd := ffmpeg.Input(inputPath).
		Output(outputPath, mp4OutputOptions).
		OverWriteOutput().
		Compile()

	// 从ffmpeg-go获取原始命令并创建exec.Cmd
	execCmd := exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)

	// 设置进程组ID，以便能够终止子进程
	execCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 捕获输出
	var stderr strings.Builder
	execCmd.Stderr = &stderr

	// 启动命令
	if err := execCmd.Start(); err != nil {
		return fmt.Errorf("启动转换进程失败: %w", err)
	}

	// 等待命令完成
	err := execCmd.Wait()

	// 检查是否超时
	if ctx.Err() == context.DeadlineExceeded {
		// 确保进程组被终止
		killProcessGroup(execCmd.Process.Pid)
		return fmt.Errorf("转换超时，已强制终止")
	}

	if err != nil {
		return fmt.Errorf("转换文件失败: %w, 错误输出: %s", err, stderr.String())
	}

	return nil
}

// killProcessGroup 终止进程组
func killProcessGroup(pid int) {
	// 发送SIGTERM信号到进程组
	if runtime.GOOS != "windows" {
		syscall.Kill(-pid, syscall.SIGTERM)

		// 给进程一些时间来优雅退出
		time.Sleep(500 * time.Millisecond)

		// 如果仍在运行，强制终止
		syscall.Kill(-pid, syscall.SIGKILL)
	} else {
		// Windows平台需要特殊处理
		exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid)).Run()
	}
}

// MergeTsToMp4 将多个TS文件直接合并为MP4文件
// 使用 ffmpeg-go 库实现
func MergeTsToMp4(tsFolder string, tsFiles []string, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	return MergeTsToMp4WithContext(ctx, tsFolder, tsFiles, outputPath)
}

// MergeTsToMp4WithContext 带上下文的TS合并函数
func MergeTsToMp4WithContext(ctx context.Context, tsFolder string, tsFiles []string, outputPath string) error {
	// 如果文件数量过多，采用分批处理策略
	const batchSize = 100 // 增大每批处理的文件数量
	if len(tsFiles) > batchSize {
		return mergeTsInBatchesWithContext(ctx, tsFolder, tsFiles, outputPath, batchSize)
	}

	// 对于小批量文件，使用常规处理方法
	// 创建临时文件以存储文件列表
	listFilePath := filepath.Join(tsFolder, "filelist.txt")
	listFile, err := os.Create(listFilePath)
	if err != nil {
		return fmt.Errorf("创建文件列表失败: %w", err)
	}
	defer func() {
		listFile.Close()
		os.Remove(listFilePath) // 清理临时文件
	}()

	// 使用缓冲写入提高文件写入速度
	buffWriter := bufio.NewWriter(listFile)

	// 写入文件列表，ffmpeg concat demuxer格式
	for _, tsFile := range tsFiles {
		tsPath := filepath.Join(tsFolder, tsFile)
		if _, err := os.Stat(tsPath); err == nil {
			// 确保路径格式正确，特别是Windows路径
			formattedPath := strings.ReplaceAll(tsPath, "\\", "/")
			_, err = buffWriter.WriteString(fmt.Sprintf("file '%s'\n", formattedPath))
			if err != nil {
				return fmt.Errorf("写入文件列表失败: %w", err)
			}
		}
	}

	// 刷新缓冲区到文件
	if err = buffWriter.Flush(); err != nil {
		return fmt.Errorf("刷新文件缓冲区失败: %w", err)
	}

	if err = listFile.Sync(); err != nil {
		return fmt.Errorf("同步文件列表失败: %w", err)
	}
	listFile.Close() // 关闭文件以便ffmpeg读取

	// 编译ffmpeg命令
	ffmpegCmd := ffmpeg.Input(listFilePath, concatOptions).
		Output(outputPath, m3u8ToMp4Options).
		OverWriteOutput()

	// 如果文件较多，显示进度
	if len(tsFiles) > 20 {
		ffmpegCmd = ffmpegCmd.GlobalArgs("-progress", "pipe:1", "-nostats")
	}

	// 获取原始命令
	cmd := ffmpegCmd.Compile()

	// 创建带上下文的命令
	execCmd := exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)
	execCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 捕获输出
	var stderr strings.Builder
	execCmd.Stderr = &stderr

	// 启动命令
	if err := execCmd.Start(); err != nil {
		return fmt.Errorf("启动合并进程失败: %w", err)
	}

	// 等待命令完成
	err = execCmd.Wait()

	// 检查是否超时
	if ctx.Err() == context.DeadlineExceeded {
		killProcessGroup(execCmd.Process.Pid)
		return fmt.Errorf("合并操作超时，已强制终止")
	}

	if err != nil {
		return fmt.Errorf("合并TS文件失败: %w, 错误输出: %s", err, stderr.String())
	}

	return nil
}

// mergeTsInBatchesWithContext 分批合并TS文件，减少内存占用，带上下文控制
func mergeTsInBatchesWithContext(ctx context.Context, tsFolder string, allTsFiles []string, finalOutputPath string, batchSize int) error {
	// 创建子上下文，用于批处理
	childCtx, childCancel := context.WithCancel(ctx)
	defer childCancel() // 确保所有子goroutine都会终止

	var tempOutputs []string
	batchCount := (len(allTsFiles) + batchSize - 1) / batchSize

	Info("文件数量过多 (%d 个文件)，采用分批处理策略，共 %d 批", len(allTsFiles), batchCount)

	// 创建临时目录存放中间文件
	tempDir := filepath.Join(tsFolder, "temp_batch_merge")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir) // 清理临时目录

	// 并行处理批次的数量，根据CPU核心数调整
	maxParallelBatches := runtime.NumCPU() / 2
	if maxParallelBatches < 1 {
		maxParallelBatches = 1
	}

	// 限制并行数不超过批次总数
	if maxParallelBatches > batchCount {
		maxParallelBatches = batchCount
	}

	// 使用通道来控制并发
	semaphore := make(chan struct{}, maxParallelBatches)
	errChan := make(chan error, batchCount)
	doneChan := make(chan int, batchCount) // 用于跟踪完成的批次
	var wg sync.WaitGroup

	// 使用预分配切片存储批次输出
	batchOutputs := make([]string, batchCount)

	// 启动监控goroutine
	monitorCtx, monitorCancel := context.WithCancel(childCtx)
	defer monitorCancel()

	go func() {
		select {
		case <-monitorCtx.Done():
			return
		case <-ctx.Done():
			// 父上下文取消，取消所有子任务
			childCancel()
			return
		}
	}()

	// 分批处理
	for i := 0; i < batchCount; i++ {
		wg.Add(1)

		// 检查上下文是否已取消
		select {
		case <-childCtx.Done():
			wg.Done()
			return fmt.Errorf("操作被取消: %v", childCtx.Err())
		case semaphore <- struct{}{}: // 获取信号量，限制并发
		}

		go func(batchIndex int) {
			defer wg.Done()
			defer func() { <-semaphore }() // 释放信号量

			// 再次检查上下文
			select {
			case <-childCtx.Done():
				errChan <- fmt.Errorf("批次%d取消: %v", batchIndex, childCtx.Err())
				return
			default:
				// 继续执行
			}

			start := batchIndex * batchSize
			end := (batchIndex + 1) * batchSize
			if end > len(allTsFiles) {
				end = len(allTsFiles)
			}

			batchFiles := allTsFiles[start:end]
			tempOutput := filepath.Join(tempDir, fmt.Sprintf("batch_%d.ts", batchIndex))

			// 线程安全地记录输出文件
			batchOutputs[batchIndex] = tempOutput

			Info("处理第 %d/%d 批 (文件 %d-%d)", batchIndex+1, batchCount, start, end-1)

			// 对每批次单独进行合并
			listFilePath := filepath.Join(tempDir, fmt.Sprintf("filelist_%d.txt", batchIndex))
			listFile, err := os.Create(listFilePath)
			if err != nil {
				errChan <- fmt.Errorf("创建批次%d文件列表失败: %w", batchIndex, err)
				return
			}

			// 使用缓冲写入提高性能
			buffWriter := bufio.NewWriter(listFile)

			// 写入文件列表
			for _, tsFile := range batchFiles {
				tsPath := filepath.Join(tsFolder, tsFile)
				if _, err := os.Stat(tsPath); err == nil {
					formattedPath := strings.ReplaceAll(tsPath, "\\", "/")
					_, err = buffWriter.WriteString(fmt.Sprintf("file '%s'\n", formattedPath))
					if err != nil {
						listFile.Close()
						errChan <- fmt.Errorf("写入批次%d文件列表失败: %w", batchIndex, err)
						return
					}
				}
			}

			// 刷新缓冲区
			if err = buffWriter.Flush(); err != nil {
				listFile.Close()
				errChan <- fmt.Errorf("刷新批次%d文件列表失败: %w", batchIndex, err)
				return
			}

			if err = listFile.Sync(); err != nil {
				listFile.Close()
				errChan <- fmt.Errorf("同步批次%d文件列表失败: %w", batchIndex, err)
				return
			}
			listFile.Close()

			// 再次检查上下文是否已取消
			select {
			case <-childCtx.Done():
				errChan <- fmt.Errorf("批次%d合并前取消: %v", batchIndex, childCtx.Err())
				return
			default:
				// 继续执行
			}

			// 编译ffmpeg命令
			ffmpegCmd := ffmpeg.Input(listFilePath, concatOptions).
				Output(tempOutput, batchOutputOptions).
				OverWriteOutput()

			cmd := ffmpegCmd.Compile()

			// 创建带上下文的命令
			batchCtx, batchCancel := context.WithTimeout(childCtx, 10*time.Minute)
			defer batchCancel()

			execCmd := exec.CommandContext(batchCtx, cmd.Args[0], cmd.Args[1:]...)
			execCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

			var stderr strings.Builder
			execCmd.Stderr = &stderr

			// 启动命令
			if startErr := execCmd.Start(); startErr != nil {
				errChan <- fmt.Errorf("启动批次%d合并进程失败: %w", batchIndex, startErr)
				return
			}

			// 等待命令完成
			err = execCmd.Wait()

			// 检查是否超时或取消
			if batchCtx.Err() != nil {
				killProcessGroup(execCmd.Process.Pid)
				errChan <- fmt.Errorf("批次%d处理被取消或超时: %v", batchIndex, batchCtx.Err())
				return
			}

			if err != nil {
				errChan <- fmt.Errorf("合并批次%d失败: %w, 错误: %s", batchIndex, err, stderr.String())
				return
			}

			// 删除这一批次的列表文件
			os.Remove(listFilePath)

			// 通知完成
			doneChan <- batchIndex
		}(i)
	}

	// 等待所有批次处理完成或出错
	go func() {
		wg.Wait()
		close(doneChan)
		close(errChan)
	}()

	// 收集错误
	var firstErr error
	for err := range errChan {
		if firstErr == nil {
			firstErr = err
			childCancel() // 一旦出错，取消所有未完成的任务
		}
	}

	// 如果有错误，返回第一个遇到的错误
	if firstErr != nil {
		return firstErr
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return fmt.Errorf("操作被取消: %v", ctx.Err())
	default:
		// 继续执行
	}

	// 添加所有成功的批次输出
	tempOutputs = batchOutputs

	// 最后合并所有临时文件为最终输出
	Info("正在合并所有批次为最终MP4文件...")

	// 创建最终合并上下文
	finalCtx, finalCancel := context.WithTimeout(ctx, 15*time.Minute)
	defer finalCancel()

	// 采用不同的合并策略，降低复杂度
	var execCmd *exec.Cmd
	var mergeErr error

	if len(tempOutputs) == 1 {
		// 只有一个批次文件，直接转换为MP4
		Info("只有一个临时文件，直接转换为MP4...")
		tempFile := tempOutputs[0]
		ffmpegCmd := ffmpeg.Input(tempFile).
			Output(finalOutputPath, mp4OutputOptions).
			OverWriteOutput()

		cmd := ffmpegCmd.Compile()
		execCmd = exec.CommandContext(finalCtx, cmd.Args[0], cmd.Args[1:]...)
	} else if len(tempOutputs) == 2 {
		// 两个批次文件，使用concat filter更可靠
		Info("使用concat filter合并两个临时文件...")

		// 确保参数中不包含可能导致问题的硬件加速
		safeOptions := CopyKwArgs(mp4OutputOptions)
		delete(safeOptions, "hwaccel")

		// 使用两个单独的Input调用
		ffmpegCmd := ffmpeg.Input(tempOutputs[0]).
			Output(finalOutputPath, safeOptions).
			OverWriteOutput()

		cmd := ffmpegCmd.Compile()
		execCmd = exec.CommandContext(finalCtx, cmd.Args[0], cmd.Args[1:]...)
	} else {
		// 尝试创建中间文件列表用于合并
		finalListPath := filepath.Join(tempDir, "final_list.txt")
		finalList, fileErr := os.Create(finalListPath)
		if fileErr != nil {
			return fmt.Errorf("创建最终文件列表失败: %w", fileErr)
		}

		// 使用缓冲写入
		buffWriter := bufio.NewWriter(finalList)
		for _, tempFile := range tempOutputs {
			formattedPath := strings.ReplaceAll(tempFile, "\\", "/")
			_, writeErr := buffWriter.WriteString(fmt.Sprintf("file '%s'\n", formattedPath))
			if writeErr != nil {
				finalList.Close()
				return fmt.Errorf("写入最终文件列表失败: %w", writeErr)
			}
		}

		if flushErr := buffWriter.Flush(); flushErr != nil {
			finalList.Close()
			return fmt.Errorf("刷新最终文件列表失败: %w", flushErr)
		}

		if syncErr := finalList.Sync(); syncErr != nil {
			finalList.Close()
			return fmt.Errorf("同步最终文件列表失败: %w", syncErr)
		}
		finalList.Close()

		// 使用更保守的参数
		safeOptions := ffmpeg.KwArgs{
			"c":       "copy",
			"threads": runtime.NumCPU(),
		}

		ffmpegCmd := ffmpeg.Input(finalListPath, concatOptions).
			Output(finalOutputPath, safeOptions).
			GlobalArgs("-v", "info").
			OverWriteOutput()

		cmd := ffmpegCmd.Compile()
		execCmd = exec.CommandContext(finalCtx, cmd.Args[0], cmd.Args[1:]...)
	}

	// 设置进程组和捕获错误输出
	execCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stderr strings.Builder
	execCmd.Stderr = &stderr

	// 启动最终合并命令
	Info("开始执行最终合并...")
	if err := execCmd.Start(); err != nil {
		return fmt.Errorf("启动最终合并进程失败: %w", err)
	}

	// 等待命令完成
	mergeErr = execCmd.Wait()

	// 检查是否超时
	if finalCtx.Err() != nil {
		killProcessGroup(execCmd.Process.Pid)
		return fmt.Errorf("最终合并操作超时或被取消: %v", finalCtx.Err())
	}

	// 如果常规合并失败，尝试备用合并方法
	if mergeErr != nil {
		Warning("常规合并失败: %v，尝试直接连接文件...", mergeErr)

		// 尝试直接连接文件（适用于某些视频格式）
		if len(tempOutputs) == 2 {
			Info("尝试直接连接二进制文件...")
			concatErr := concatenateBinaryFiles(tempOutputs, finalOutputPath)
			if concatErr != nil {
				return fmt.Errorf("合并失败: %w，连接文件也失败: %v", mergeErr, concatErr)
			} else {
				Info("使用二进制连接成功！")
			}
		} else {
			return fmt.Errorf("合并多个批次文件失败: %w, 错误: %s", mergeErr, stderr.String())
		}
	}

	Info("合并完成!")

	return nil
}

// concatenateBinaryFiles 直接连接二进制文件（适用于某些格式，仅作为最后的备选方案）
func concatenateBinaryFiles(inputFiles []string, outputPath string) error {
	// 创建输出文件
	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer output.Close()

	// 依次读取每个输入文件并写入输出文件
	bufSize := 4 * 1024 * 1024 // 4MB缓冲区
	buf := make([]byte, bufSize)

	for _, file := range inputFiles {
		input, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("打开输入文件%s失败: %w", file, err)
		}

		// 复制文件内容
		for {
			n, err := input.Read(buf)
			if err != nil && err != io.EOF {
				input.Close()
				return fmt.Errorf("读取文件%s失败: %w", file, err)
			}
			if n == 0 {
				break
			}

			if _, err := output.Write(buf[:n]); err != nil {
				input.Close()
				return fmt.Errorf("写入输出文件失败: %w", err)
			}
		}

		input.Close()
	}

	return nil
}

// DirectMergeFromM3u8 直接从M3U8 URL合并为MP4文件
func DirectMergeFromM3u8(m3u8URL, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	return DirectMergeFromM3u8WithContext(ctx, m3u8URL, outputPath)
}

// DirectMergeFromM3u8WithContext 带上下文的M3U8直接下载
func DirectMergeFromM3u8WithContext(ctx context.Context, m3u8URL, outputPath string) error {
	// 准备增强的输入选项
	enhancedInputOptions := CopyKwArgs(m3u8InputOptions)

	// 增加网络相关优化参数
	enhancedInputOptions["timeout"] = "60000000"         // 超时时间60秒(微秒单位)
	enhancedInputOptions["rw_timeout"] = "60000000"      // 读写超时
	enhancedInputOptions["stimeout"] = "60000000"        // 流媒体超时
	enhancedInputOptions["analyzeduration"] = "10000000" // 分析时长10秒(微秒)
	enhancedInputOptions["probesize"] = "32000000"       // 提高探测大小到32M

	// 自定义下载参数
	customOptions := CopyKwArgs(m3u8ToMp4Options)
	customOptions["segment_time_delta"] = "0.1" // 片段时间容差值

	// 自动判断是否需要显示进度
	ffmpegCmd := ffmpeg.Input(m3u8URL, enhancedInputOptions).
		Output(outputPath, customOptions).
		GlobalArgs("-stats").
		OverWriteOutput()

	// 编译命令
	cmd := ffmpegCmd.Compile()

	// 开始下载
	startTime := time.Now()
	Info("开始从M3U8下载视频: %s", m3u8URL)

	// 尝试最多3次
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// 创建有超时的上下文
		attemptCtx, attemptCancel := context.WithTimeout(ctx, 30*time.Minute)

		if attempt > 1 {
			Info("第%d次重试下载...", attempt)
			// 每次重试前等待一段时间
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		// 创建命令
		execCmd := exec.CommandContext(attemptCtx, cmd.Args[0], cmd.Args[1:]...)
		execCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		var stderr strings.Builder
		execCmd.Stderr = &stderr

		// 启动命令
		if startErr := execCmd.Start(); startErr != nil {
			attemptCancel()
			err = fmt.Errorf("启动下载进程失败: %w", startErr)
			continue
		}

		// 等待命令完成
		err = execCmd.Wait()

		// 检查是否超时
		if attemptCtx.Err() != nil {
			killProcessGroup(execCmd.Process.Pid)
			attemptCancel()
			err = fmt.Errorf("下载操作超时或被取消: %v", attemptCtx.Err())
			continue
		}

		attemptCancel()

		if err == nil {
			break // 成功则退出循环
		}

		Warning("下载尝试 %d/%d 失败: %v\n错误输出: %s", attempt, maxRetries, err, stderr.String())
	}

	if err != nil {
		return fmt.Errorf("直接从M3U8合并失败(重试%d次): %w", maxRetries, err)
	}

	// 计算总用时
	duration := time.Since(startTime)
	Info("下载完成，总用时: %s", duration.String())

	return nil
}

// MergeTsToMp4WithFfmpegGo 保持旧的接口以兼容现有代码
func MergeTsToMp4WithFfmpegGo(tsFolder string, tsFiles []string, outputPath string) error {
	return MergeTsToMp4(tsFolder, tsFiles, outputPath)
}

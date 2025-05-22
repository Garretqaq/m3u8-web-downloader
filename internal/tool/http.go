package tool

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/ratelimit"
)

// 全局 HTTP 客户端（使用连接池）
var (
	clientOnce sync.Once
	httpClient *http.Client
)

// 全局下载限速器相关变量
var (
	globalLimiter          ratelimit.Limiter
	globalLimiterLock      sync.Mutex               // 保护 globalLimiter 和 globalLimiterSpeedKBps 的更新
	globalLimiterSpeedKBps int64                    // 当前全局限速器的设置值 (KB/s)
	isGlobalLimiterEnabled bool                     // 标记全局限速器是否启用
	lastConfigTime         time.Time                // 上次配置限速器的时间
	disableSignal          = make(chan struct{}, 1) // 用于通知所有读取器禁用限速的信号
)

// RateLimitedReader 使用uber-go/ratelimit包实现漏桶限速的读取器
type RateLimitedReader struct {
	r               io.ReadCloser
	limiter         ratelimit.Limiter // 将引用全局限速器或独立的限速器
	readBytes       int64             // 已读取的总字节数
	startTime       time.Time         // 开始时间，用于计算平均速度
	speedKBpsForLog int64             // 用于日志记录的限速值
	buf             []byte            // 读取缓冲区，减少小块读取
}

// RefreshGlobalRateLimiter 强制刷新全局限速器状态
// 当怀疑限速器设置没有被应用时调用此函数
func RefreshGlobalRateLimiter() {
	globalLimiterLock.Lock()
	defer globalLimiterLock.Unlock()

	// 如果限速器没有启用，不进行任何操作
	if !isGlobalLimiterEnabled || globalLimiter == nil {
		Debug("[全局限速] 刷新状态: 当前未启用限速")
		return
	}

	// 重新创建限速器，使用相同的设置
	speed := globalLimiterSpeedKBps
	bytesPerSecond := speed * 1024
	chunkSize := int64(64)
	opsPerSecond := bytesPerSecond / chunkSize

	if opsPerSecond < 10 {
		opsPerSecond = 10
	}

	// 记录刷新操作
	timeSinceLastConfig := time.Since(lastConfigTime).Seconds()
	Debug("[全局限速] 强制刷新状态: %d KB/s (距上次配置: %.1f秒)",
		speed, timeSinceLastConfig)

	// 重新创建限速器
	globalLimiter = ratelimit.New(int(opsPerSecond))
	lastConfigTime = time.Now()
}

// ConfigureGlobalRateLimiter 配置或更新全局下载限速器
// speedKBps: >0 表示设置限速值 (KB/s); <=0 表示禁用全局限速
func ConfigureGlobalRateLimiter(speedKBps int64) {
	globalLimiterLock.Lock()
	defer globalLimiterLock.Unlock()

	// 是否从启用状态变为禁用状态
	wasEnabled := isGlobalLimiterEnabled

	if speedKBps <= 0 {
		if isGlobalLimiterEnabled {
			Debug("[全局限速] 已禁用")

			// 如果之前是启用状态，现在禁用，发送信号通知所有读取器
			if wasEnabled {
				// 尝试清空信号通道 (非阻塞方式)
				select {
				case <-disableSignal:
				default:
				}

				// 发送新的禁用信号
				select {
				case disableSignal <- struct{}{}:
					Debug("[全局限速] 已发送全局禁用信号")
				default:
					Debug("[全局限速] 禁用信号通道已满，无法发送")
				}
			}
		}
		globalLimiter = nil
		globalLimiterSpeedKBps = 0
		isGlobalLimiterEnabled = false
		lastConfigTime = time.Now()
		return
	}

	// 如果限速值未改变且限速器已存在，就简单记录一下但仍然重新创建限速器
	// 这样可以确保多次设置相同值也能正确响应和重置限速器状态
	if isGlobalLimiterEnabled && globalLimiterSpeedKBps == speedKBps && globalLimiter != nil {
		Debug("[全局限速] 重新应用相同限速值: %d KB/s", speedKBps)
	}

	// 修改令牌桶计算方式，考虑多线程并发下载
	// 将原本按KB计算改为按字节计算，更精确控制速率
	// 1KB = 1024字节，换算成每秒操作数
	bytesPerSecond := speedKBps * 1024

	// 设置小粒度的令牌生成频率，提高精度
	// 例如使用64字节的小块，可以更精确地控制速率
	chunkSize := int64(64) // 每个操作单位的字节数
	opsPerSecond := bytesPerSecond / chunkSize

	if opsPerSecond < 10 { // 确保最小限制
		opsPerSecond = 10
	}

	// 始终创建一个新的限速器，确保配置生效
	globalLimiter = ratelimit.New(int(opsPerSecond))
	globalLimiterSpeedKBps = speedKBps
	isGlobalLimiterEnabled = true
	lastConfigTime = time.Now()
	Debug("[全局限速] 已配置: %d KB/s (每秒操作数: %d, 块大小: %d 字节)",
		speedKBps, opsPerSecond, chunkSize)
}

// newSharedRateLimitedReader 创建一个使用指定限速器的 RateLimitedReader
func newSharedRateLimitedReader(r io.ReadCloser, limiter ratelimit.Limiter, speedForLogKBps int64) *RateLimitedReader {
	return &RateLimitedReader{
		r:               r,
		limiter:         limiter,
		readBytes:       0,
		startTime:       time.Now(),
		speedKBpsForLog: speedForLogKBps,
		buf:             make([]byte, 256), // 使用256B缓冲区
	}
}

// Read 实现读取并应用限速
func (r *RateLimitedReader) Read(p []byte) (n int, err error) {
	// 如果读取器自己的限速器为nil，则不限速
	if r.limiter == nil {
		return r.r.Read(p)
	}

	// 快速检查禁用信号
	select {
	case <-disableSignal:
		// 收到禁用信号，不再限速
		Debug("[限速读取器] 收到全局禁用信号，当前读取不限速")
		return r.r.Read(p)
	default:
		// 未收到禁用信号，继续检查全局状态
	}

	// 检查全局限速器状态
	globalLimiterLock.Lock()
	limiterEnabled := isGlobalLimiterEnabled
	activeLimiter := r.limiter
	globalLimiterLock.Unlock()

	// 如果全局禁用了限速，直接读取不限速
	if !limiterEnabled {
		return r.r.Read(p)
	}

	toRead := len(p)
	if toRead == 0 {
		return 0, nil
	}
	totalRead := 0

	// 修改为更小块的读取单位，提高限速精度
	chunkSize := 64 // 使用64字节的小块进行限速

	for totalRead < toRead {
		// 每次循环前都快速检查禁用信号
		select {
		case <-disableSignal:
			// 收到禁用信号，读取剩余部分不限速
			bytesRead, readErr := r.r.Read(p[totalRead:])
			totalRead += bytesRead
			return totalRead, readErr
		default:
			// 未收到禁用信号，继续限速读取
		}

		// 再次检查全局状态，确保最新状态
		globalLimiterLock.Lock()
		limiterEnabled = isGlobalLimiterEnabled
		globalLimiterLock.Unlock()

		// 如果限速已被禁用，直接读取剩余部分
		if !limiterEnabled {
			bytesRead, readErr := r.r.Read(p[totalRead:])
			totalRead += bytesRead
			return totalRead, readErr
		}

		currentChunkSize := chunkSize
		if currentChunkSize > toRead-totalRead {
			currentChunkSize = toRead - totalRead
		}

		// 每读取一个块，就消耗一个令牌
		activeLimiter.Take()

		bytesRead, readErr := r.r.Read(p[totalRead : totalRead+currentChunkSize])
		totalRead += bytesRead

		if bytesRead > 0 {
			r.readBytes += int64(bytesRead)
			// 每读取约512KB数据记录一次日志
			if r.readBytes >= 512*1024 && r.readBytes%(512*1024) < int64(bytesRead) {
				elapsed := time.Since(r.startTime).Seconds()
				if elapsed > 0 {
					avgSpeed := float64(r.readBytes) / elapsed / 1024.0
					logPrefix := "[全局限速]"

					// 重新获取最新的限速值用于日志显示
					globalLimiterLock.Lock()
					currentLimit := globalLimiterSpeedKBps
					globalLimiterLock.Unlock()

					Debug("%s 平均下载速度: %.2f KB/s (当前设定: %d KB/s)",
						logPrefix, avgSpeed, currentLimit)
				}
			}
		}

		if readErr != nil {
			return totalRead, readErr
		}
	}
	return totalRead, nil
}

func (r *RateLimitedReader) Close() error {
	return r.r.Close()
}

// initHTTPClient 创建带连接池的全局客户端
func initHTTPClient() {
	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		DisableCompression: false,
	}

	httpClient = &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
	}
}

// Get 获取 URL 内容。如果全局限速器已配置并启用，则下载将受其限制。
func Get(url string) (io.ReadCloser, error) {
	clientOnce.Do(initHTTPClient)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close() // 确保在非200状态码时关闭body
		return nil, fmt.Errorf("http error: status code %d", resp.StatusCode)
	}

	globalLimiterLock.Lock()
	currentLimiter := globalLimiter
	currentSpeed := globalLimiterSpeedKBps
	isEnabled := isGlobalLimiterEnabled
	globalLimiterLock.Unlock()

	if isEnabled && currentLimiter != nil {
		// 增加更详细的调试信息，显示当前活跃并发下载和总限速关系
		// 这可以帮助排查多线程下载时的限速问题
		Debug("[全局限速] 应用全局限速 %d KB/s 到 URL: %s (分片64字节)",
			currentSpeed, url)

		// 使用更精确的限速读取器
		return newSharedRateLimitedReader(resp.Body, currentLimiter, currentSpeed), nil
	}

	return resp.Body, nil
}

// Debug function stub (assuming it exists elsewhere or will be added)
// func Debug(format string, args ...interface{}) {
// 	// Example: log.Printf("DEBUG: " + format, args...)
// }

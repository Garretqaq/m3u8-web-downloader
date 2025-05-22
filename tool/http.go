package tool

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"m3u8-go/config"
)

// 全局用于控制下载速度的结构
var (
	speedLimitMutex sync.Mutex // 保护令牌桶
	tokenBucket     int64      // 当前可用的字节数
	lastFillTime    time.Time  // 上次填充时间
	clientOnce      sync.Once
	httpClient      *http.Client
)

// 限速读取器
type RateLimitedReader struct {
	r          io.ReadCloser
	lastCheck  time.Time
	checkEvery int64 // 每读取多少字节检查一次速度限制
}

// 创建一个新的限速读取器
func NewRateLimitedReader(r io.ReadCloser) *RateLimitedReader {
	return &RateLimitedReader{
		r:          r,
		lastCheck:  time.Now(),
		checkEvery: 8 * 1024, // 每读取8KB检查一次
	}
}

func (r *RateLimitedReader) Read(p []byte) (n int, err error) {
	// 动态读取配置中的速度限制
	limitKBps := config.Get().DownloadSpeedLimit

	// 未限速直接读取
	if limitKBps <= 0 {
		return r.r.Read(p)
	}

	// 确定这次能读取多少字节
	toRead := len(p)

	// 令牌桶算法实现限速
	speedLimitMutex.Lock()
	defer speedLimitMutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(lastFillTime).Seconds()

	// 计算这段时间内应该添加多少令牌
	newTokens := int64(float64(limitKBps) * 1024 * elapsed)

	// 添加令牌，但不超过桶的容量(1秒的容量)
	maxBucketSize := int64(limitKBps) * 1024
	tokenBucket = min(tokenBucket+newTokens, maxBucketSize)
	lastFillTime = now

	// 计算本次可以读取的字节数
	allowedBytes := min(int64(toRead), tokenBucket)

	if allowedBytes <= 0 {
		// 没有足够令牌，需要等待
		wait := time.Duration(float64(1000) / float64(limitKBps) * float64(toRead) / 1024.0 * float64(time.Millisecond))
		if wait > 500*time.Millisecond {
			wait = 500 * time.Millisecond // 最多等待500ms
		}
		time.Sleep(wait)
		return 0, nil // 返回0表示本次没有读取数据，但也不是错误
	}

	// 限制本次读取的字节数
	n, err = r.r.Read(p[:allowedBytes])

	// 减去使用的令牌
	if n > 0 {
		tokenBucket -= int64(n)
	}

	return n, err
}

func (r *RateLimitedReader) Close() error {
	return r.r.Close()
}

// min returns the minimum of two int64 values
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
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

// Get 获取URL内容，带限速功能
func Get(url string) (io.ReadCloser, error) {
	clientOnce.Do(initHTTPClient)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error: status code %d", resp.StatusCode)
	}

	// 包装响应体为限速读取器
	return NewRateLimitedReader(resp.Body), nil
}

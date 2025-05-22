package tool

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// 全局 HTTP 客户端（使用连接池）
var (
	clientOnce sync.Once
	httpClient *http.Client
)

// RateLimitedReader 使用令牌桶算法对读取速度做限制
type RateLimitedReader struct {
	r           io.ReadCloser
	speedKBps   int64 // 速度限制 KB/s，<=0 表示不限速
	tokenBucket int64 // 当前可用字节数
	lastFill    time.Time
}

// NewRateLimitedReader 返回带速度限制的读取器
func NewRateLimitedReader(r io.ReadCloser, speedKBps int64) *RateLimitedReader {
	return &RateLimitedReader{
		r:           r,
		speedKBps:   speedKBps,
		tokenBucket: speedKBps * 1024, // 初始放满 1s 的令牌
		lastFill:    time.Now(),
	}
}

func (r *RateLimitedReader) Read(p []byte) (n int, err error) {
	// 未限速
	if r.speedKBps <= 0 {
		return r.r.Read(p)
	}

	toRead := len(p)

	// 令牌桶填充
	now := time.Now()
	elapsed := now.Sub(r.lastFill).Seconds()
	r.tokenBucket += int64(float64(r.speedKBps) * 1024 * elapsed)

	maxBucket := r.speedKBps * 1024
	if r.tokenBucket > maxBucket {
		r.tokenBucket = maxBucket
	}
	r.lastFill = now

	allowed := min(int64(toRead), r.tokenBucket)
	if allowed <= 0 {
		// 没有足够令牌，短暂休眠后重试
		time.Sleep(50 * time.Millisecond)
		return 0, nil
	}

	n, err = r.r.Read(p[:allowed])
	if n > 0 {
		r.tokenBucket -= int64(n)
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

// ------------------------- Option 机制 -------------------------

type Option func(*options)

type options struct {
	speedKBps int64
}

// WithRateLimit 指定下载速度限制，单位 KB/s；<=0 表示不限速
func WithRateLimit(speedKBps int64) Option {
	return func(o *options) {
		o.speedKBps = speedKBps
	}
}

// Get 获取 URL 内容；可通过 Option 指定额外行为（如速度限制）
func Get(url string, opts ...Option) (io.ReadCloser, error) {
	clientOnce.Do(initHTTPClient)

	// 解析 Option
	var opt options
	for _, f := range opts {
		f(&opt)
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error: status code %d", resp.StatusCode)
	}

	// 根据 Option 决定是否限速
	if opt.speedKBps > 0 {
		return NewRateLimitedReader(resp.Body, opt.speedKBps), nil
	}
	return resp.Body, nil
}

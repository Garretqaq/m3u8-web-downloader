package tool

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 日志级别常量
const (
	LogLevelDebug = iota
	LogLevelInfo
	LogLevelWarning
	LogLevelError
)

// 获取东八区时区
var cstZone = time.FixedZone("CST", 8*3600) // 东八区，即UTC+8

// 日志相关变量
var (
	// 当前日志级别，默认为Info
	currentLogLevel = LogLevelInfo

	// 不同日志级别的前缀
	debugPrefix   = "[DEBUG] "
	infoPrefix    = "[INFO] "
	warningPrefix = "[WARN] "
	errorPrefix   = "[ERROR] "
)

// 格式化时间戳为东八区时间
func formatTimeCST(t time.Time) string {
	return t.In(cstZone).Format("2006/01/02 15:04:05.000")
}

// 初始化日志系统
func init() {
	// 尝试从IANA时区数据库加载亚洲/上海时区
	if zone, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		cstZone = zone
	}
}

// SetLogLevel 设置当前日志级别
func SetLogLevel(level int) {
	currentLogLevel = level
}

// Debug 打印调试信息
func Debug(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelDebug {
		fmt.Printf("%s %s%s\n", formatTimeCST(time.Now()), debugPrefix, fmt.Sprintf(format, args...))
	}
}

// Info 打印普通信息
func Info(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelInfo {
		fmt.Printf("%s %s%s\n", formatTimeCST(time.Now()), infoPrefix, fmt.Sprintf(format, args...))
	}
}

// Warning 打印警告信息
func Warning(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelWarning {
		fmt.Printf("%s %s%s\n", formatTimeCST(time.Now()), warningPrefix, fmt.Sprintf(format, args...))
	}
}

// Error 打印错误信息
func Error(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelError {
		fmt.Fprintf(os.Stderr, "%s %s%s\n", formatTimeCST(time.Now()), errorPrefix, fmt.Sprintf(format, args...))
	}
}

// 暂存需要清理的临时文件/目录
var tempFiles []string
var tempMutex sync.Mutex

// AddTempFile 添加一个需要在程序退出时清理的临时文件或目录
func AddTempFile(path string) {
	tempMutex.Lock()
	defer tempMutex.Unlock()
	tempFiles = append(tempFiles, path)
}

// Cleanup 清理所有临时文件和目录
func Cleanup() {
	tempMutex.Lock()
	defer tempMutex.Unlock()

	for _, path := range tempFiles {
		// 尝试删除文件或目录，忽略错误
		_ = os.RemoveAll(path)
	}
	tempFiles = nil
}

func CurrentDir(joinPath ...string) (string, error) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", err
	}
	p := strings.Replace(dir, "\\", "/", -1)
	whole := filepath.Join(joinPath...)
	whole = filepath.Join(p, whole)
	return whole, nil
}

func ResolveURL(u *url.URL, p string) string {
	if strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "http://") {
		return p
	}
	var baseURL string
	if strings.Index(p, "/") == 0 {
		baseURL = u.Scheme + "://" + u.Host
	} else {
		tU := u.String()
		baseURL = tU[0:strings.LastIndex(tU, "/")]
	}
	return baseURL + path.Join("/", p)
}

func DrawProgressBar(prefix string, proportion float32, width int, suffix ...string) {
	pos := int(proportion * float32(width))
	s := fmt.Sprintf("%s%*s %6.2f%% %s",
		strings.Repeat("■", pos), width-pos, "", proportion*100, strings.Join(suffix, ""))

	// 直接打印进度条，不使用日志记录
	fmt.Printf("\r[%s] %s", prefix, s)
}

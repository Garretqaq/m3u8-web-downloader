package tool

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// 日志级别常量
const (
	LogLevelDebug = iota
	LogLevelInfo
	LogLevelWarning
	LogLevelError
)

// 日志相关变量
var (
	// 当前日志级别，默认为Info
	currentLogLevel = LogLevelInfo

	// 各级别的logger
	debugLogger   *log.Logger
	infoLogger    *log.Logger
	warningLogger *log.Logger
	errorLogger   *log.Logger
)

// 初始化日志系统
func init() {
	// 设置标准库日志格式：显示日期、时间、毫秒和时区
	logFlags := log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC

	// 创建不同级别的logger
	debugLogger = log.New(os.Stdout, "[DEBUG] ", logFlags)
	infoLogger = log.New(os.Stdout, "[INFO] ", logFlags)
	warningLogger = log.New(os.Stdout, "[WARN] ", logFlags)
	errorLogger = log.New(os.Stderr, "[ERROR] ", logFlags)
}

// SetLogLevel 设置当前日志级别
func SetLogLevel(level int) {
	currentLogLevel = level
}

// Debug 打印调试信息
func Debug(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelDebug {
		debugLogger.Printf(format, args...)
	}
}

// Info 打印普通信息
func Info(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelInfo {
		infoLogger.Printf(format, args...)
	}
}

// Warning 打印警告信息
func Warning(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelWarning {
		warningLogger.Printf(format, args...)
	}
}

// Error 打印错误信息
func Error(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelError {
		errorLogger.Printf(format, args...)
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

	// 使用Info记录进度条信息而不是直接打印
	if currentLogLevel <= LogLevelInfo {
		fmt.Printf("\r[%s] %s", prefix, s)
	}
}

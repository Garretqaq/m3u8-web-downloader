package tool

import (
	"fmt"
	"time"
)

// 日志级别常量
const (
	LogLevelDebug = iota
	LogLevelInfo
	LogLevelWarning
	LogLevelError
)

var (
	// 当前日志级别，默认为Info
	currentLogLevel = LogLevelInfo
)

// SetLogLevel 设置当前日志级别
func SetLogLevel(level int) {
	currentLogLevel = level
}

// Debug 打印调试信息
func Debug(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelDebug {
		logWithLevel("DEBUG", format, args...)
	}
}

// Info 打印普通信息
func Info(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelInfo {
		logWithLevel("INFO", format, args...)
	}
}

// Warning 打印警告信息
func Warning(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelWarning {
		logWithLevel("WARN", format, args...)
	}
}

// Error 打印错误信息
func Error(format string, args ...interface{}) {
	if currentLogLevel <= LogLevelError {
		logWithLevel("ERROR", format, args...)
	}
}

// logWithLevel 带日志级别和时间戳的日志打印
func logWithLevel(level string, format string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] [%s] %s\n", timestamp, level, message)
}

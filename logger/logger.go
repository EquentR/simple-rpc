package logger

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel 日志级别类型
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// ParseLevel 将字符串解析为LogLevel
func ParseLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// String 返回日志级别的字符串表示
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger 日志接口，可由外部实现替换默认日志
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// DefaultLogger 默认日志实现，使用标准库log包
type DefaultLogger struct {
	level LogLevel
}

// NewDefaultLogger 创建默认日志实例
func NewDefaultLogger() *DefaultLogger {
	return &DefaultLogger{
		level: INFO, // 默认日志级别为INFO
	}
}

// SetLevel 设置日志级别
func (l *DefaultLogger) SetLevel(level LogLevel) {
	l.level = level
}

// getLevel 获取日志级别
func (l *DefaultLogger) getLevel() LogLevel {
	return l.level
}

func (l *DefaultLogger) log(level LogLevel, format string, args ...interface{}) {
	// 检查日志级别是否满足要求
	if level < l.level {
		return
	}

	levelStr := level.String()

	// 获取调用者信息
	pc, file, line, ok := runtime.Caller(3) // 跳过两层调用栈：log -> Debug/Info/Warn/Error -> 实际调用处
	if !ok {
		file = "unknown"
		line = 0
	} else {
		// 简化文件路径，只保留最后的文件名
		if idx := strings.LastIndex(file, "/"); idx != -1 {
			file = file[idx+1:]
		}
	}

	// 获取函数名
	funcName := "unknown"
	if fn := runtime.FuncForPC(pc); fn != nil {
		fullFuncName := fn.Name()
		if idx := strings.LastIndex(fullFuncName, "/"); idx != -1 {
			funcName = fullFuncName[idx+1:]
		} else {
			funcName = fullFuncName
		}
	}

	// 格式化时间
	timestamp := time.Now().Format("2006/01/02 15:04:05")

	// 构造日志消息
	message := fmt.Sprintf(format, args...)
	logEntry := fmt.Sprintf("[%s] %-5s %s:%d %s: %s\n", timestamp, levelStr, file, line, funcName, message)

	// 根据日志级别选择输出流
	switch level {
	case DEBUG:
		fmt.Print(logEntry)
	case INFO:
		fmt.Print(logEntry)
	case WARN:
		fmt.Print(logEntry)
	case ERROR:
		fmt.Fprint(os.Stderr, logEntry)
	}
}

func (l *DefaultLogger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

func (l *DefaultLogger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

func (l *DefaultLogger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

func (l *DefaultLogger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

var globalLogger Logger = NewDefaultLogger()

// SetLogger 设置全局日志实例
func SetLogger(l Logger) {
	globalLogger = l
}

// GetLogger 获取当前日志实例
func GetLogger() Logger {
	return globalLogger
}

// SetLevel 设置全局日志级别
func SetLevel(level LogLevel) {
	if defaultLogger, ok := globalLogger.(*DefaultLogger); ok {
		defaultLogger.SetLevel(level)
	}
}

// Debug 全局调试日志
func Debug(format string, args ...interface{}) {
	globalLogger.Debug(format, args...)
}

// Info 全局信息日志
func Info(format string, args ...interface{}) {
	globalLogger.Info(format, args...)
}

// Warn 全局警告日志
func Warn(format string, args ...interface{}) {
	globalLogger.Warn(format, args...)
}

// Error 全局错误日志
func Error(format string, args ...interface{}) {
	globalLogger.Error(format, args...)
}

package logger

import (
	"log"
	"os"
)

// Logger 日志接口，可由外部实现替换默认日志
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// DefaultLogger 默认日志实现，使用标准库log包
type DefaultLogger struct {
	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	error *log.Logger
}

// NewDefaultLogger 创建默认日志实例
func NewDefaultLogger() *DefaultLogger {
	return &DefaultLogger{
		debug: log.New(os.Stdout, "[DEBUG] ", log.LstdFlags|log.Lshortfile),
		info:  log.New(os.Stdout, "[INFO] ", log.LstdFlags|log.Lshortfile),
		warn:  log.New(os.Stdout, "[WARN] ", log.LstdFlags|log.Lshortfile),
		error: log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lshortfile),
	}
}

func (l *DefaultLogger) Debug(format string, args ...interface{}) {
	l.debug.Printf(format, args...)
}

func (l *DefaultLogger) Info(format string, args ...interface{}) {
	l.info.Printf(format, args...)
}

func (l *DefaultLogger) Warn(format string, args ...interface{}) {
	l.warn.Printf(format, args...)
}

func (l *DefaultLogger) Error(format string, args ...interface{}) {
	l.error.Printf(format, args...)
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

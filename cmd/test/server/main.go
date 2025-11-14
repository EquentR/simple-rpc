package main

import (
	"fmt"
	"github.com/EquentR/simple-rpc/internal/tlsutil"
	"github.com/EquentR/simple-rpc/logger"
	"github.com/EquentR/simple-rpc/rpc"
	"github.com/EquentR/simple-rpc/server"
	"time"
)

// 定义路由常量
const (
	RouteHealth = "/health"
	RouteStream = "/stream"
)

func main() {
	// 创建TLS配置
	_, cfg, err := tlsutil.SelfSigned([]string{"127.0.0.1"})
	if err != nil {
		logger.Error("Failed to create TLS config: %v", err)
		panic(err)
	}

	// 创建服务器实例
	srv := server.New("127.0.0.1:8443", cfg, 8)

	// 注册健康检查路由 - 请求响应模式
	srv.Mux.Handle(RouteHealth, handleHealth)
	// 注册流式消息路由
	srv.Mux.Handle(RouteStream, handleStream)

	if err := srv.Serve(); err != nil {
		logger.Error("Server failed: %v", err)
		panic(err)
	}
}

func handleStream(ctx *rpc.Context) {
	receive, err := ctx.ReceiveMessageBlocking()
	if err != nil {
		logger.Error("Failed to receive stream message: %v", err)
		return
	}
	logger.Info("Received stream message: %s", receive)
	err = ctx.SendMessage([]byte("Hello, World!"))
	if err != nil {
		logger.Error("Failed to send stream message: %v", err)
		return
	}
	receive, err = ctx.ReceiveMessageBlocking()
	if err != nil {
		logger.Error("Failed to receive stream message: %v", err)
		return
	}
	logger.Info("Received stream message: %s", receive)
	if err := ctx.SendMessage([]byte("Goodbye, World!")); err != nil {
		logger.Error("Failed to send stream message: %v", err)
		return
	}
}

// handleHealth 健康检查处理器
func handleHealth(ctx *rpc.Context) {
	logger.Debug("Health check request received")

	responseBytes := []byte(fmt.Sprintf(`{"status":"healthy","timestamp":%d,"service":"simple-rpc-server","version":"1.0.0"}`,
		time.Now().Unix()))

	if err := ctx.Reply(responseBytes); err != nil {
		logger.Error("Failed to send health response: %v", err)
	} else {
		logger.Debug("Health check response sent successfully")
	}
}

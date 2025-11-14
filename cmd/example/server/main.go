package main

import (
	"fmt"
	"time"

	"github.com/EquentR/simple-rpc/internal/tlsutil"
	"github.com/EquentR/simple-rpc/logger"
	"github.com/EquentR/simple-rpc/rpc"
	"github.com/EquentR/simple-rpc/server"
)

// 定义路由常量
const (
	RouteHealth     = "/health"
	RouteEcho       = "/echo"
	RouteStreamTime = "/stream/time"
)

func main() {
	logger.Info("Starting best practice RPC server with 2 request-response routes and 1 streaming route")

	// 创建TLS配置
	_, cfg, err := tlsutil.SelfSigned([]string{"127.0.0.1"})
	if err != nil {
		logger.Error("Failed to create TLS config: %v", err)
		panic(err)
	}

	// 创建服务器实例
	srv := server.New("127.0.0.1:8446", cfg, 8)

	// 注册健康检查路由 - 请求响应模式
	srv.Mux.Handle(RouteHealth, handleHealth)

	// 注册回显路由 - 请求响应模式
	srv.Mux.Handle(RouteEcho, handleEcho)

	// 注册时间流路由 - 真正的流式处理
	srv.Mux.Handle(RouteStreamTime, handleStreamTime)

	logger.Info("Server configured with routes:")
	logger.Info("  %s - Health check (request-response)", RouteHealth)
	logger.Info("  %s - Echo service (request-response)", RouteEcho)
	logger.Info("  %s - Time streaming service (true streaming)", RouteStreamTime)

	// 启动服务器
	logger.Info("Starting server on 127.0.0.1:8446")
	if err := srv.Serve(); err != nil {
		logger.Error("Server failed: %v", err)
		panic(err)
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

// handleEcho 回显处理器
func handleEcho(ctx *rpc.Context) {
	logger.Debug("Echo request received: %s", string(ctx.Payload))

	// 创建回显响应
	echoResponse := fmt.Sprintf(`{"echo":"%s","timestamp":%d}`,
		string(ctx.Payload), time.Now().Unix())

	if err := ctx.Reply([]byte(echoResponse)); err != nil {
		logger.Error("Failed to send echo response: %v", err)
	} else {
		logger.Debug("Echo response sent successfully")
	}
}

// handleStreamTime 时间流处理器 - 真正的流式处理
func handleStreamTime(ctx *rpc.Context) {
	logger.Info("Starting time streaming session")

	// 获取流写入器
	stream := ctx.Stream()

	// 发送初始消息
	initialMsg := fmt.Sprintf(`{"type":"init","message":"Time stream started","timestamp":%d}`, time.Now().Unix())
	if err := stream.WriteChunk([]byte(initialMsg), false); err != nil {
		logger.Error("Failed to send initial stream message: %v", err)
		return
	}

	// 持续发送时间更新（真正的流式处理）
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	messageCount := 0
	maxMessages := 10

	for {
		select {
		case <-ticker.C:
			messageCount++

			// 创建时间消息
			timeMsg := fmt.Sprintf(`{"type":"time_update","count":%d,"timestamp":%d,"formatted_time":"%s"}`,
				messageCount, time.Now().Unix(), time.Now().Format(time.RFC3339))

			// 检查是否是最后一条消息
			isLast := messageCount >= maxMessages

			logger.Debug("Sending time update %d/%d", messageCount, maxMessages)

			// 发送流式消息
			if err := stream.WriteChunk([]byte(timeMsg), isLast); err != nil {
				logger.Error("Failed to send time stream message: %v", err)
				return
			}

			// 如果是最后一条消息，结束流
			if isLast {
				logger.Info("Time streaming completed - sent %d messages", messageCount)
				return
			}
		}
	}
}

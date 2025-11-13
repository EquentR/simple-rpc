package main

import (
	"fmt"
	"github.com/EquentR/simple-rpc/internal/tlsutil"
	"github.com/EquentR/simple-rpc/logger"
	"github.com/EquentR/simple-rpc/rpc"
	"github.com/EquentR/simple-rpc/server"
	"time"
)

func main() {
	logger.Info("Starting complex TCP server with bidirectional streaming support")

	_, cfg, err := tlsutil.SelfSigned([]string{"127.0.0.1"})
	if err != nil {
		panic(err)
	}

	s := server.New("127.0.0.1:8443", cfg, 8)

	// 基本回显处理函数，使用不同的路由
	s.Mux.Handle("/echo-0", func(ctx *rpc.Context) {
		logger.Info("Handling echo-0 request, payload: %s", string(ctx.Payload))
		response := fmt.Sprintf("Echo-0 response: %s", string(ctx.Payload))
		_ = ctx.Reply([]byte(response))
	})

	s.Mux.Handle("/echo-1", func(ctx *rpc.Context) {
		logger.Info("Handling echo-1 request, payload: %s", string(ctx.Payload))
		response := fmt.Sprintf("Echo-1 response: %s", string(ctx.Payload))
		_ = ctx.Reply([]byte(response))
	})

	s.Mux.Handle("/echo-2", func(ctx *rpc.Context) {
		logger.Info("Handling echo-2 request, payload: %s", string(ctx.Payload))
		response := fmt.Sprintf("Echo-2 response: %s", string(ctx.Payload))
		_ = ctx.Reply([]byte(response))
	})

	// 服务器到客户端的流式传输
	s.Mux.Handle("/stream", func(ctx *rpc.Context) {
		logger.Info("Starting server-to-client streaming")
		w := ctx.Stream()
		for i := 0; i < 5; i++ {
			chunk := fmt.Sprintf("Server stream chunk %d at %s", i, time.Now().Format(time.RFC3339))
			logger.Info("Sending stream chunk %d", i)
			_ = w.WriteChunk([]byte(chunk), false)
			time.Sleep(300 * time.Millisecond)
		}
		_ = w.WriteChunk([]byte("Stream completed"), true)
		logger.Info("Server-to-client streaming completed")
	})

	// 双向流式处理函数
	s.Mux.Handle("/bidirectional", func(ctx *rpc.Context) {
		logger.Info("Starting bidirectional streaming session")
		w := ctx.Stream()

		// 发送初始服务器流
		_ = w.WriteChunk([]byte("Server: Bidirectional stream initialized"), false)

		// 模拟服务器在客户端发送独立请求时发送数据
		for i := 0; i < 3; i++ {
			serverMsg := fmt.Sprintf("Server streaming message %d", i)
			logger.Info("Sending server stream message %d", i)
			_ = w.WriteChunk([]byte(serverMsg), false)
			time.Sleep(1 * time.Second)
		}

		_ = w.WriteChunk([]byte("Server: Bidirectional stream ended"), true)
		logger.Info("Bidirectional streaming session completed")
	})

	// 客户端流式处理函数
	s.Mux.Handle("/client-stream", func(ctx *rpc.Context) {
		logger.Info("Processing client streaming message: %s", string(ctx.Payload))
		response := fmt.Sprintf("Server acknowledged: %s", string(ctx.Payload))
		_ = ctx.Reply([]byte(response))
	})

	// 基于会话的流式处理函数
	for sessionID := 0; sessionID < 3; sessionID++ {
		sessionID := sessionID // 捕获循环变量
		route := fmt.Sprintf("/stream-session-%d", sessionID)

		s.Mux.Handle(route, func(ctx *rpc.Context) {
			logger.Info("Session %d received message: %s", sessionID, string(ctx.Payload))
			response := fmt.Sprintf("Session %d acknowledged at %s", sessionID, time.Now().Format(time.RFC3339))
			_ = ctx.Reply([]byte(response))
		})
	}

	// 连接监控处理函数
	s.Mux.Handle("/status", func(ctx *rpc.Context) {
		logger.Info("Status check requested")
		status := fmt.Sprintf("Server running - Active connections: %d - Time: %s", getActiveConnectionCount(), time.Now().Format(time.RFC3339))
		_ = ctx.Reply([]byte(status))
	})

	logger.Info("Server configured with multiple handlers, starting to serve")

	if err := s.Serve(); err != nil {
		logger.Error("Server failed: %v", err)
		panic(err)
	}
}

func getActiveConnectionCount() int {
	// 这是一个模拟函数 - 在实际实现中，你需要跟踪连接
	return 5 + int(time.Now().Unix()%10)
}

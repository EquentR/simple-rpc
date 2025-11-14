package main

import (
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/EquentR/simple-rpc/client"
	"github.com/EquentR/simple-rpc/logger"
)

// 定义路由常量（与服务端保持一致）
const (
	RouteHealth     = "/health"
	RouteEcho       = "/echo"
	RouteStreamTime = "/stream/time"
)

func main() {
	logger.Info("Starting best practice RPC client example")

	// 创建TLS配置（跳过证书验证用于测试）
	cfg := &tls.Config{InsecureSkipVerify: true}

	// 创建客户端连接池
	pool := client.New("127.0.0.1:8446", cfg, 3)
	defer pool.Close()

	logger.Info("Client connected to server at 127.0.0.1:8446")
	logger.Info("Connection pool size: 3")

	// 使用WaitGroup协调并发操作
	var wg sync.WaitGroup

	// 测试1: 健康检查（请求-响应模式）
	wg.Add(1)
	go func() {
		defer wg.Done()
		testHealthCheck(pool)
	}()

	// 测试2: 回显服务（请求-响应模式）
	wg.Add(1)
	go func() {
		defer wg.Done()
		testEchoService(pool)
	}()

	// 测试3: 时间流服务（流式处理）
	wg.Add(1)
	go func() {
		defer wg.Done()
		testTimeStreaming(pool)
	}()

	// 等待所有测试完成
	wg.Wait()
	logger.Info("All client tests completed successfully")
}

// testHealthCheck 测试健康检查功能
func testHealthCheck(pool *client.ClientPool) {
	logger.Info("=== Testing Health Check ===")

	// 发送健康检查请求
	payload := []byte(`{"action":"check"}`)
	response, err := pool.Call(RouteHealth, payload, 5*time.Second)

	if err != nil {
		logger.Error("Health check failed: %v", err)
		return
	}

	logger.Info("Health check response: %s", string(response))

	// 模拟多次健康检查
	for i := 0; i < 3; i++ {
		response, err := pool.Call(RouteHealth, payload, 3*time.Second)
		if err != nil {
			logger.Error("Health check %d failed: %v", i+1, err)
			continue
		}
		logger.Info("Health check %d: %s", i+1, string(response))
		time.Sleep(500 * time.Millisecond)
	}

	logger.Info("=== Health Check Test Completed ===")
}

// testEchoService 测试回显服务功能
func testEchoService(pool *client.ClientPool) {
	logger.Info("=== Testing Echo Service ===")

	// 测试不同的回显消息
	testMessages := []string{
		"Hello, Server!",
		"Testing echo service",
		"Special characters: 你好世界 🌍",
		"JSON data: {\"key\":\"value\",\"number\":42}",
	}

	for i, message := range testMessages {
		payload := []byte(message)
		response, err := pool.Call(RouteEcho, payload, 3*time.Second)

		if err != nil {
			logger.Error("Echo test %d failed: %v", i+1, err)
			continue
		}

		logger.Info("Echo %d - Sent: '%s', Received: '%s'", i+1, message, string(response))
		time.Sleep(200 * time.Millisecond)
	}

	logger.Info("=== Echo Service Test Completed ===")
}

// testTimeStreaming 测试时间流服务功能
func testTimeStreaming(pool *client.ClientPool) {
	logger.Info("=== Testing Time Streaming Service ===")

	// 发送流请求
	payload := []byte(`{"action":"start_stream"}`)
	streamChan, err := pool.CallStream(RouteStreamTime, payload, 15*time.Second)

	if err != nil {
		logger.Error("Failed to start time streaming: %v", err)
		return
	}

	logger.Info("Time streaming started, waiting for messages...")

	// 接收流式消息
	messageCount := 0
	startTime := time.Now()

	for data := range streamChan {
		messageCount++
		logger.Info("Stream message %d: %s", messageCount, string(data))

		// 显示接收进度
		if messageCount%3 == 0 {
			elapsed := time.Since(startTime)
			logger.Info("Received %d messages in %.1f seconds", messageCount, elapsed.Seconds())
		}
	}

	totalElapsed := time.Since(startTime)
	logger.Info("=== Time Streaming Completed ===")
	logger.Info("Total messages received: %d", messageCount)
	logger.Info("Total duration: %.1f seconds", totalElapsed.Seconds())
	logger.Info("Average message rate: %.1f messages/second", float64(messageCount)/totalElapsed.Seconds())
}

// testConcurrentRequests 测试并发请求（可选的高级测试）
func testConcurrentRequests(pool *client.ClientPool) {
	logger.Info("=== Testing Concurrent Requests ===")

	var wg sync.WaitGroup
	concurrency := 5
	requestsPerWorker := 4

	startTime := time.Now()

	// 启动多个并发工作线程
	for workerID := 0; workerID < concurrency; workerID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			logger.Debug("Worker %d started", id)

			for reqID := 0; reqID < requestsPerWorker; reqID++ {
				// 随机选择路由进行测试
				var route string
				var payload []byte

				if reqID%2 == 0 {
					route = RouteHealth
					payload = []byte(fmt.Sprintf(`{"worker":%d,"request":%d}`, id, reqID))
				} else {
					route = RouteEcho
					payload = []byte(fmt.Sprintf("Worker %d - Request %d", id, reqID))
				}

				response, err := pool.Call(route, payload, 2*time.Second)
				if err != nil {
					logger.Error("Worker %d request %d failed: %v", id, reqID, err)
					continue
				}

				logger.Debug("Worker %d request %d: %s", id, reqID, string(response))

				// 小延迟避免过载
				time.Sleep(50 * time.Millisecond)
			}

			logger.Debug("Worker %d completed", id)
		}(workerID)
	}

	// 等待所有工作线程完成
	wg.Wait()

	totalElapsed := time.Since(startTime)
	totalRequests := concurrency * requestsPerWorker

	logger.Info("=== Concurrent Requests Test Completed ===")
	logger.Info("Total requests: %d", totalRequests)
	logger.Info("Total duration: %.2f seconds", totalElapsed.Seconds())
	logger.Info("Average requests per second: %.1f", float64(totalRequests)/totalElapsed.Seconds())
}

package main

import (
	"crypto/tls"
	"fmt"
	"github.com/EquentR/simple-rpc/client"
	"github.com/EquentR/simple-rpc/logger"
	"sync"
	"time"
)

func main() {
	logger.Info("Starting complex TCP client example with bidirectional streaming and connection pool simulation")

	cfg := &tls.Config{InsecureSkipVerify: true}
	pool := client.New("127.0.0.1:8443", cfg, 5)
	defer pool.Close()

	var wg sync.WaitGroup

	// 模拟连接池增长和回收
	wg.Add(1)
	go func() {
		defer wg.Done()
		wgg := sync.WaitGroup{}
		logger.Info("Starting connection pool simulation - dynamic size adjustment")

		// 使用小连接池的初始请求
		for i := 0; i < 3; i++ {
			wgg.Add(1)
			go func(iteration int) {
				defer wgg.Done()
				logger.Info("Request iteration %d with initial pool size", iteration)
				performRequestResponse(pool, iteration)
			}(i)
		}

		time.Sleep(2 * time.Second)

		// 扩展连接池大小
		logger.Info("Expanding connection pool from 5 to 8 connections")
		pool.SetSize(8)

		// 使用扩展后的连接池进行更多请求
		for i := 3; i < 6; i++ {
			wgg.Add(1)
			go func(iteration int) {
				defer wgg.Done()
				logger.Info("Request iteration %d with expanded pool size", iteration)
				performRequestResponse(pool, iteration)
			}(i)
		}

		time.Sleep(2 * time.Second)

		// 缩小连接池大小
		logger.Info("Shrinking connection pool from 8 to 3 connections")
		pool.SetSize(3)

		// 使用缩减后的连接池进行最终请求
		for i := 6; i < 10; i++ {
			wgg.Add(1)
			go func(iteration int) {
				defer wgg.Done()
				logger.Info("Request iteration %d with shrunk pool size", iteration)
				performRequestResponse(pool, iteration)
			}(i)
		}
		wgg.Wait()
	}()

	// 双向流式示例
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Second) // 先让一些请求完成
		logger.Info("Starting bidirectional streaming example")
		performBidirectionalStreaming(pool)
	}()

	// 多个并发流式会话
	wg.Add(1)
	go func() {
		defer wg.Done()
		wgg := sync.WaitGroup{}
		time.Sleep(3 * time.Second)
		logger.Info("Starting multiple concurrent streaming sessions")
		for sessionID := 0; sessionID < 3; sessionID++ {
			wgg.Add(1)
			go func(sid int) {
				defer wgg.Done()
				logger.Info("Starting concurrent streaming session %d", sid)
				performClientStreaming(pool, sid)
			}(sessionID)
		}
		wgg.Wait()
	}()

	wg.Wait()
	logger.Info("Complex client example completed")
}

func performRequestResponse(pool *client.ClientPool, iteration int) {
	route := fmt.Sprintf("/echo-%d", iteration%3)
	payload := []byte(fmt.Sprintf("Hello from iteration %d at %s", iteration, time.Now().Format(time.RFC3339)))

	logger.Info("Sending request-response call to route %s, iteration %d", route, iteration)

	response, err := pool.Call(route, payload, 5*time.Second)
	if err != nil {
		logger.Error("Request failed for iteration %d: %v", iteration, err)
		return
	}

	logger.Info("Received response for iteration %d: %s", iteration, string(response))

	// 模拟处理时间
	time.Sleep(time.Duration(100+iteration*50) * time.Millisecond)
}

func performBidirectionalStreaming(pool *client.ClientPool) {
	logger.Info("Starting bidirectional streaming session")

	// 首先，建立服务器到客户端的流式传输
	streamCh, err := pool.CallStream("/bidirectional", []byte("init"), 10*time.Second)
	if err != nil {
		logger.Error("Failed to establish bidirectional stream: %v", err)
		return
	}

	// 处理传入的流数据
	incomingDone := make(chan struct{})
	go func() {
		defer close(incomingDone)
		for data := range streamCh {
			logger.Info("Received server stream data: %s", string(data))
		}
		logger.Info("Server-to-client stream ended")
	}()

	// 发送多个客户端到服务器的流式请求
	for i := 0; i < 5; i++ {
		streamPayload := []byte(fmt.Sprintf("Client streaming message %d", i))
		logger.Info("Sending client stream message %d", i)

		// 为每个客户端流消息使用独立的连接
		_, err := pool.Call("/client-stream", streamPayload, 3*time.Second)
		if err != nil {
			logger.Error("Client stream message %d failed: %v", i, err)
		} else {
			logger.Info("Client stream message %d sent successfully", i)
		}

		time.Sleep(500 * time.Millisecond)
	}

	// 等待传入流完成
	<-incomingDone
	logger.Info("Bidirectional streaming session completed")
}

func performClientStreaming(pool *client.ClientPool, sessionID int) {
	logger.Info("Starting client streaming session %d", sessionID)

	// 发送多个流式消息
	for msgID := 0; msgID < 4; msgID++ {
		payload := []byte(fmt.Sprintf("Session %d - Message %d - Timestamp: %d", sessionID, msgID, time.Now().UnixNano()))
		route := fmt.Sprintf("/stream-session-%d", sessionID)

		logger.Info("Session %d sending message %d", sessionID, msgID)

		response, err := pool.Call(route, payload, 2*time.Second)
		if err != nil {
			logger.Error("Session %d message %d failed: %v", sessionID, msgID, err)
			continue
		}

		logger.Info("Session %d received response for message %d: %s", sessionID, msgID, string(response))

		// 消息之间的变化延迟
		time.Sleep(time.Duration(200+msgID*100) * time.Millisecond)
	}

	logger.Info("Client streaming session %d completed", sessionID)
}

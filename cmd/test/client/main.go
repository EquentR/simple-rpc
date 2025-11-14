package main

import (
	"crypto/tls"
	"github.com/EquentR/simple-rpc/client"
	"github.com/EquentR/simple-rpc/logger"
	"sync"
	"time"
)

// 定义路由常量
const (
	RouteHealth = "/health"
	RouteStream = "/stream"
)

func main() {
	// 创建TLS配置（跳过证书验证用于测试）
	cfg := &tls.Config{InsecureSkipVerify: true}

	// 创建客户端连接池
	pool := client.New("127.0.0.1:8443", cfg, 3)
	defer pool.Close()

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			health, err := pool.Call(RouteHealth, []byte(""), 3*time.Second)
			if err != nil {
				panic(err)
			}
			logger.Info("Health check response: %s", health)
		}()
	}
	wg.Wait()

	session, err := pool.CreateBidirectionalSession(RouteStream, nil)
	if err != nil {
		panic(err)
	}
	err = session.Send([]byte("Hello, server!"))
	if err != nil {
		panic(err)
	}
	blocking, err := session.ReceiveBlocking()
	if err != nil {
		panic(err)
	}
	logger.Info("Received stream: %s", blocking)
	err = session.Send([]byte("Bye, server!"))
	if err != nil {
		panic(err)
	}
	blocking, err = session.ReceiveBlocking()
	if err != nil {
		panic(err)
	}
	logger.Info("Received stream: %s", blocking)
	session.Close()
}

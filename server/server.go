// Package server 提供TCP服务器实现
// 包括连接管理、路由、工作池等功能
package server

import (
	"crypto/tls"
	"github.com/EquentR/simple-rpc/conn"
	"github.com/EquentR/simple-rpc/logger"
	"github.com/EquentR/simple-rpc/protocol"
	"github.com/EquentR/simple-rpc/rpc"
	"sync"
	"time"
)

// Mux 是路由多路复用器，管理路由到处理函数的映射
type Mux struct {
	m map[string]rpc.Handler // 路由到处理函数的映射
}

// NewMux 创建新的路由多路复用器
func NewMux() *Mux { return &Mux{m: map[string]rpc.Handler{}} }

// Handle 注册路由处理函数
func (mx *Mux) Handle(path string, h rpc.Handler) { mx.m[path] = h }

// get 获取指定的路由处理函数
func (mx *Mux) get(path string) rpc.Handler { return mx.m[path] }

// WorkerPool 管理处理请求的goroutine
type WorkerPool struct {
	ch   chan *rpc.Context // 任务通道
	stop chan struct{}     // 停止信号
}

// NewWorkerPool 创建工作池
func NewWorkerPool(size int) *WorkerPool {
	return &WorkerPool{ch: make(chan *rpc.Context, size*2), stop: make(chan struct{})}
}

// Start 启动工作池，创建指定数量的工作goroutine
func (p *WorkerPool) Start(size int) {
	logger.Info("Starting worker pool, worker goroutine count: %d", size)
	for i := 0; i < size; i++ {
		go func(workerID int) {
			logger.Debug("Worker goroutine %d started", workerID)
			for {
				select {
				case ctx := <-p.ch:
					if ctx != nil && ctx.H != nil {
						logger.Debug("Worker goroutine %d processing request, route: %s", workerID, ctx.Route)
						ctx.H(ctx)
					}
				case <-p.stop:
					logger.Debug("Worker goroutine %d exiting", workerID)
					return
				}
			}
		}(i)
	}
}

// Submit 提交任务到工作池
func (p *WorkerPool) Submit(ctx *rpc.Context) { p.ch <- ctx }

// Stop 停止工作池
func (p *WorkerPool) Stop() {
	logger.Info("Stopping worker pool")
	close(p.stop)
}

// Conn 类型别名，使用conn.Connection
type Conn = conn.Connection

// Server TCP服务器
type Server struct {
	Addr      string      // 服务器监听地址
	TLSConfig *tls.Config // TLS配置
	Mux       *Mux        // 路由多路复用器
	Pool      *WorkerPool // 工作池
	conns     sync.Map    // Active connection mapping
}

// New 创建新的TCP服务器
func New(addr string, cfg *tls.Config, workers int) *Server {
	logger.Info("Creating TCP server, address: %s, worker pool size: %d", addr, workers)
	s := &Server{Addr: addr, TLSConfig: cfg, Mux: NewMux(), Pool: NewWorkerPool(workers)}
	s.Pool.Start(workers)
	return s
}

// Serve 启动服务器，开始监听和处理连接
func (s *Server) Serve() error {
	logger.Info("Server starting to listen, address: %s", s.Addr)

	// 创建TLS监听器
	ln, err := tls.Listen("tcp", s.Addr, s.TLSConfig)
	if err != nil {
		logger.Error("Failed to create listener: %v", err)
		return err
	}

	logger.Info("Server listening successfully, waiting for client connections")

	// Accept connection loop
	for {
		c, err := ln.Accept()
		if err != nil {
			logger.Error("Failed to accept connection: %v", err)
			return err
		}

		logger.Info("Accepted new connection, remote address: %s", c.RemoteAddr())

		// Wrap connection and store
		cn := conn.NewFrom(c)
		s.conns.Store(cn, cn)

		// Start processing goroutine
		go s.handleConn(cn)
	}
}

// handleConn 处理单个连接的所有请求
func (s *Server) handleConn(cn *Conn) {
	logger.Debug("Starting connection handling, remote address processing")

	// Ensure resource cleanup when connection closes
	defer func() {
		cn.Close()
		s.conns.Delete(cn)
		logger.Debug("Connection handling ended")
	}()

	// Request processing loop
	for {
		// Read request frame
		f, err := cn.ReadOne()
		if err != nil {
			logger.Debug("Failed to read frame, connection may be closed: %v", err)
			return
		}

		logger.Debug("Received frame, ID: %d, flags: %d, type length: %d, data length: %d",
			f.ID, f.Flags, len(f.Type), len(f.Value))

		// Only process request frames
		if f.Flags&protocol.FlagRequest == 0 {
			logger.Warn("Received non-request frame, ignoring")
			continue
		}

		// Parse route
		route := string(f.Type)
		h := s.Mux.get(route)

		// Create processing context
		ctx := rpc.NewContext(cn, f.ID, route, f.Value, h)

		// If no handler found, return empty response
		if h == nil {
			logger.Warn("Route handler not found: %s", route)
			_ = ctx.Reply([]byte{})
			continue
		}

		// Submit to worker pool for processing
		s.Pool.Submit(ctx)
		logger.Debug("Request submitted to worker pool, route: %s", route)
	}
}

// Shutdown 优雅地关闭服务器
func (s *Server) Shutdown(d time.Duration) {
	logger.Info("Starting to shutdown server")

	// Stop worker pool
	s.Pool.Stop()
	logger.Info("Worker pool stopped")

	// Close all active connections
	closedCount := 0
	s.conns.Range(func(k, v any) bool {
		if c, ok := v.(*Conn); ok {
			c.Close()
			closedCount++
		}
		return true
	})

	logger.Info("Closed %d active connections", closedCount)
	logger.Info("Server shutdown completed")
}

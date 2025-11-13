package client

import (
	"crypto/tls"
	"errors"
	"github.com/EquentR/simple-rpc/conn"
	"github.com/EquentR/simple-rpc/logger"
	"github.com/EquentR/simple-rpc/protocol"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ClientPool 管理连接池中的多个TCP连接
type ClientPool struct {
	Addr        string        // 服务器地址
	TLSConfig   *tls.Config   // TLS配置
	Size        int           // 连接池大小
	conns       []*Conn       // 连接数组
	idx         atomic.Uint32 // 负载均衡的轮询索引
	rid         atomic.Uint64 // 请求ID生成器
	mu          sync.RWMutex  // 读写锁保护连接池操作
	DialTimeout time.Duration // 连接超时时间
	connIDGen   atomic.Uint64 // 连接ID生成器
}

// Conn 包装客户端连接
type Conn struct {
	id   uint64           // 连接唯一ID
	addr string           // 服务器地址
	cfg  *tls.Config      // TLS配置
	cn   *conn.Connection // 底层连接
	resp sync.Map         // 响应通道映射表
	dead atomic.Bool      // 连接死亡标志
	p    *ClientPool      // 父连接池
}

// New 创建新的客户端连接池
func New(addr string, cfg *tls.Config, size int) *ClientPool {
	logger.Info("Creating client connection pool, address: %s, size: %d", addr, size)
	p := &ClientPool{Addr: addr, TLSConfig: cfg, Size: size, DialTimeout: 3 * time.Second}
	p.conns = make([]*Conn, size)
	for i := 0; i < size; i++ {
		connID := p.connIDGen.Add(1)
		p.conns[i] = &Conn{id: connID, addr: addr, cfg: cfg, p: p}
		logger.Debug("Creating connection object, connection ID: %d, index: %d", connID, i)
	}
	return p
}

// ensure 确保连接可用，如果关闭则重新连接
func (cc *Conn) ensure() error {
	if cc.cn != nil && !cc.cn.Closed() && !cc.dead.Load() {
		logger.Debug("Connection ID: %d exists and is available", cc.id)
		return nil
	}

	if cc.cn != nil {
		logger.Debug("Connection ID: %d closed, preparing to reconnect", cc.id)
		cc.cn.Close()
	}

	logger.Info("Connection ID: %d starting TCP connection establishment, address: %s", cc.id, cc.addr)
	d := &net.Dialer{Timeout: cc.p.DialTimeout}
	c, err := tls.DialWithDialer(d, "tcp", cc.addr, cc.cfg)
	if err != nil {
		logger.Error("Connection ID: %d TCP connection establishment failed: %v", cc.id, err)
		return err
	}

	cc.cn = conn.NewFrom(c)
	cc.dead.Store(false)
	logger.Info("Connection ID: %d TCP connection established successfully", cc.id)

	// 启动读取goroutine
	go cc.read()
	return nil
}

// read 持续从连接读取数据帧
func (cc *Conn) read() {
	logger.Debug("Connection ID: %d read goroutine started", cc.id)
	for {
		f, err := cc.cn.ReadOne()
		if err != nil {
			logger.Error("Connection ID: %d read data failed: %v", cc.id, err)
			cc.dead.Store(true)
			return
		}

		logger.Debug("Connection ID: %d received data frame, ID: %d, type: %s", cc.id, f.ID, string(f.Type))

		chv, ok := cc.resp.Load(f.ID)
		if !ok {
			logger.Warn("Connection ID: %d no channel found for request ID: %d", cc.id, f.ID)
			continue
		}

		ch := chv.(chan *protocol.Frame)
		ch <- f

		// 如果是非流式响应或流式响应结束，关闭通道并清理映射
		if f.Flags&protocol.FlagStream == 0 || f.Flags&protocol.FlagStreamEnd != 0 {
			logger.Debug("Connection ID: %d closing response channel for request ID: %d", cc.id, f.ID)
			close(ch)
			cc.resp.Delete(f.ID)
		}
	}
}

// write 发送数据帧
func (cc *Conn) write(f *protocol.Frame) error {
	logger.Debug("Connection ID: %d preparing to send data frame, request ID: %d, type: %s", cc.id, f.ID, string(f.Type))

	if err := cc.ensure(); err != nil {
		logger.Error("Connection ID: %d failed to ensure connection availability: %v", cc.id, err)
		return err
	}

	err := cc.cn.WriteFrame(f)
	if err != nil {
		logger.Error("Connection ID: %d failed to send data frame: %v", cc.id, err)
		cc.dead.Store(true)
	} else {
		logger.Debug("Connection ID: %d successfully sent data frame, request ID: %d", cc.id, f.ID)
	}
	return err
}

// next 从连接池获取下一个可用连接（轮询算法）
//
// 工作流程：
// 1. 使用原子操作递增轮询索引，实现简单的负载均衡
// 2. 检查获取的连接是否有效（非空、未死亡、底层连接未关闭）
// 3. 如果连接无效，在写锁保护下重新创建连接对象
// 4. 返回可用连接
//
// 注意事项：
// - 使用读写锁提高并发性能
// - 无效连接自动重新创建，无需手动处理
// - 返回nil表示连接池为空
func (p *ClientPool) next() *Conn {
	logger.Debug("Starting to get next available connection from pool")

	// 读取当前连接池状态
	p.mu.RLock()
	n := len(p.conns)
	if n == 0 {
		p.mu.RUnlock()
		logger.Warn("Connection pool is empty, cannot get connection")
		return nil
	}

	// 使用原子操作递增索引，实现轮询负载均衡
	i := int(p.idx.Add(1)) % n
	cc := p.conns[i]
	p.mu.RUnlock()

	logger.Debug("Round-robin getting connection, index: %d, connection ID: %d", i, cc.id)

	// 检查连接是否有效
	if cc == nil || cc.dead.Load() || (cc.cn != nil && cc.cn.Closed()) {
		logger.Info("Connection ID: %d invalid, preparing to recreate", cc.id)

		// 获取写锁以重新创建连接
		p.mu.Lock()
		// 双重检查确保连接仍然无效
		if i < len(p.conns) && (p.conns[i] == nil || p.conns[i].dead.Load() || (p.conns[i].cn != nil && p.conns[i].cn.Closed())) {
			connID := p.connIDGen.Add(1)
			p.conns[i] = &Conn{id: connID, addr: p.Addr, cfg: p.TLSConfig, p: p}
			logger.Info("Connection pool index %d recreating connection, new connection ID: %d", i, connID)
		}
		cc = p.conns[i]
		p.mu.Unlock()
	}

	logger.Debug("Successfully obtained connection, connection ID: %d", cc.id)
	return cc
}

// GetConnID 获取当前连接ID
func (cc *Conn) GetConnID() uint64 {
	return cc.id
}

// Call 进行同步远程服务调用
func (p *ClientPool) Call(route string, payload []byte, timeout time.Duration) ([]byte, error) {
	logger.Info("Starting synchronous call, route: %s, timeout: %v", route, timeout)

	// 生成请求ID
	id := p.rid.Add(1)
	logger.Debug("Generated request ID: %d", id)

	// 从连接池获取可用连接
	cc := p.next()
	if cc == nil {
		logger.Error("Cannot get available connection")
		return nil, errors.New("no connection")
	}

	logger.Debug("Using connection ID: %d for call", cc.GetConnID())

	// 创建响应通道
	ch := make(chan *protocol.Frame, 1)
	cc.resp.Store(id, ch)
	logger.Debug("Storing response channel, request ID: %d", id)

	// 构建请求帧
	f := &protocol.Frame{Flags: protocol.FlagRequest, ID: id, Type: []byte(route), Value: payload}

	// 发送请求
	if err := cc.write(f); err != nil {
		logger.Error("Failed to send request: %v", err)
		cc.resp.Delete(id)
		return nil, err
	}

	// 等待响应或超时
	t := time.NewTimer(timeout)
	defer t.Stop()

	select {
	case rf := <-ch:
		logger.Debug("Received response, request ID: %d", id)
		if rf.Flags&protocol.FlagError != 0 {
			errMsg := string(rf.Value)
			logger.Error("Server returned error: %s", errMsg)
			return nil, errors.New(errMsg)
		}
		logger.Info("Synchronous call successful, route: %s", route)
		return rf.Value, nil
	case <-t.C:
		logger.Error("Request timeout, request ID: %d", id)
		cc.resp.Delete(id)
		return nil, errors.New("timeout")
	}
}

// CallStream 进行流式远程服务调用，返回数据通道
func (p *ClientPool) CallStream(route string, payload []byte, timeout time.Duration) (<-chan []byte, error) {
	logger.Info("Starting streaming call, route: %s, timeout: %v", route, timeout)

	// 生成请求ID
	id := p.rid.Add(1)
	logger.Debug("Generated streaming request ID: %d", id)

	// 从连接池获取可用连接
	cc := p.next()
	if cc == nil {
		logger.Error("Cannot get available connection for streaming call")
		return nil, errors.New("no connection")
	}

	logger.Debug("Using connection ID: %d for streaming call", cc.GetConnID())

	// 创建内部响应通道和外部数据通道
	ch := make(chan *protocol.Frame, 4)
	out := make(chan []byte, 4)
	cc.resp.Store(id, ch)
	logger.Debug("Storing streaming response channel, request ID: %d", id)

	// 构建请求帧
	f := &protocol.Frame{Flags: protocol.FlagRequest, ID: id, Type: []byte(route), Value: payload}

	// 发送请求
	if err := cc.write(f); err != nil {
		logger.Error("Streaming call failed to send request: %v", err)
		cc.resp.Delete(id)
		close(out)
		return nil, err
	}

	// 启动处理goroutine
	t := time.NewTimer(timeout)
	go func() {
		logger.Debug("Starting streaming response processing goroutine, request ID: %d", id)
		defer func() {
			close(out)
			logger.Debug("Streaming response processing goroutine ended, request ID: %d", id)
		}()

		for {
			select {
			case rf, ok := <-ch:
				if !ok {
					logger.Debug("Streaming response channel closed, request ID: %d", id)
					return
				}
				if rf.Flags&protocol.FlagError != 0 {
					logger.Error("Streaming call server returned error, request ID: %d", id)
					return
				}
				logger.Debug("Streaming call received data, request ID: %d, data length: %d", id, len(rf.Value))
				out <- rf.Value
				// 检查流式响应是否结束
				if rf.Flags&protocol.FlagStream == 0 || rf.Flags&protocol.FlagStreamEnd != 0 {
					logger.Info("Streaming call response ended, request ID: %d", id)
					return
				}
			case <-t.C:
				logger.Error("Streaming call timeout, request ID: %d", id)
				return
			}
		}
	}()

	logger.Info("Streaming call started successfully, route: %s", route)
	return out, nil
}

// SetSize 动态调整连接池大小
func (p *ClientPool) SetSize(size int) {
	if size < 0 {
		logger.Warn("Attempting to set invalid connection pool size: %d", size)
		return
	}

	logger.Info("Adjusting connection pool size, target size: %d", size)

	p.mu.Lock()
	defer p.mu.Unlock()

	cur := len(p.conns)
	if size == cur {
		logger.Debug("Connection pool size no adjustment needed, current size: %d", cur)
		return
	}

	if size < cur {
		logger.Info("Shrinking connection pool from %d to %d", cur, size)
		// 关闭多余的连接
		for i := size; i < cur; i++ {
			if p.conns[i] != nil && p.conns[i].cn != nil {
				logger.Debug("Closing connection at pool index %d", i)
				p.conns[i].cn.Close()
			}
		}
		p.conns = p.conns[:size]
		p.Size = size
		logger.Info("Connection pool shrink completed")
		return
	}

	// 扩展连接池
	addCount := size - cur
	logger.Info("Expanding connection pool from %d to %d, adding %d connections", cur, size, addCount)

	add := make([]*Conn, addCount)
	for i := 0; i < len(add); i++ {
		connID := p.connIDGen.Add(1)
		add[i] = &Conn{id: connID, addr: p.Addr, cfg: p.TLSConfig, p: p}
		logger.Debug("Creating new connection, connection ID: %d", connID)
	}
	p.conns = append(p.conns, add...)
	p.Size = size
	logger.Info("Connection pool expansion completed")
}

// Close 关闭连接池并释放所有连接资源
func (p *ClientPool) Close() {
	logger.Info("Starting to close connection pool, address: %s", p.Addr)

	p.mu.Lock()
	defer p.mu.Unlock()

	// 关闭所有连接
	for i, cc := range p.conns {
		if cc != nil && cc.cn != nil {
			logger.Debug("Closing connection at pool index %d, connection ID: %d", i, cc.id)
			cc.cn.Close()
		}
	}

	logger.Info("Connection pool close completed")
}

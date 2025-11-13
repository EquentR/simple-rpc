// Package conn 提供TCP连接封装
// 包含连接管理、帧读写等功能
package conn

import (
	"github.com/EquentR/simple-rpc/protocol"
	"net"
	"sync"
	"sync/atomic"
)

// Connection TCP连接封装
type Connection struct {
	c      net.Conn    // 底层网络连接
	wmu    sync.Mutex  // 写操作互斥锁，保证写操作的原子性
	closed atomic.Bool // 连接关闭状态标记
}

// NewFrom 从现有的net.Conn创建Connection
func NewFrom(c net.Conn) *Connection { return &Connection{c: c} }

// WriteFrame 发送协议帧
// 使用写锁保证并发安全
func (cn *Connection) WriteFrame(f *protocol.Frame) error {
	// 检查连接是否已关闭
	if cn.closed.Load() {
		return net.ErrClosed
	}

	// 编码帧数据
	b, err := protocol.EncodeFrame(f)
	if err != nil {
		return err
	}

	// 获取写锁，保证写操作的原子性
	cn.wmu.Lock()
	defer cn.wmu.Unlock()

	// 写入数据
	_, err = cn.c.Write(b)
	if err != nil {
		// 写失败时标记连接为关闭状态
		cn.closed.Store(true)
	}
	return err
}

// ReadOne 读取一个协议帧
// 如果读取失败，会自动标记连接为关闭状态
func (cn *Connection) ReadOne() (*protocol.Frame, error) {
	// 从底层连接解码帧数据
	f, err := protocol.DecodeFrom(cn.c)
	if err != nil {
		// 读取失败时标记连接为关闭状态
		cn.closed.Store(true)
		return nil, err
	}
	return f, nil
}

// Close 关闭连接
func (cn *Connection) Close() {
	cn.closed.Store(true)
	_ = cn.c.Close()
}

// Closed 检查连接是否已关闭
func (cn *Connection) Closed() bool {
	return cn.closed.Load()
}

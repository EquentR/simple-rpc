// Package rpc 定义RPC调用相关的类型和接口
// 包含请求上下文、响应写入器、处理函数等
package rpc

import (
	"net"
	"sync/atomic"

	"github.com/EquentR/simple-rpc/conn"
	"github.com/EquentR/simple-rpc/logger"
	"github.com/EquentR/simple-rpc/protocol"
)

// Handler RPC请求处理函数类型
type Handler func(*Context)

// ResponseWriter 响应写入器，用于发送响应数据
type ResponseWriter struct {
	conn     *conn.Connection     // 连接对象
	id       uint64               // 请求ID
	route    []byte               // 路由信息
	done     atomic.Bool          // 写入完成标记
	incoming chan *protocol.Frame // 传入消息通道（用于双向通信）
}

// NewResponseWriter 创建新的响应写入器
func NewResponseWriter(cn *conn.Connection, id uint64, route []byte) *ResponseWriter {
	return &ResponseWriter{
		conn:     cn,
		id:       id,
		route:    route,
		incoming: make(chan *protocol.Frame, 100), // 缓冲通道避免阻塞
	}
}

// Write 写入完整响应数据
func (w *ResponseWriter) Write(b []byte) error {
	if w.done.Load() {
		logger.Warn("Response already written, duplicate write, request ID: %d", w.id)
		return net.ErrClosed
	}

	logger.Debug("Writing response data, request ID: %d, data length: %d", w.id, len(b))

	// 构建响应帧
	f := &protocol.Frame{Flags: protocol.FlagResponse, ID: w.id, Type: w.route, Value: b}
	if err := w.conn.WriteFrame(f); err != nil {
		logger.Error("Failed to write response, request ID: %d: %v", w.id, err)
		return err
	}

	w.done.Store(true)
	logger.Debug("Response write completed, request ID: %d", w.id)
	return nil
}

// WriteChunk 写入流式响应数据块
func (w *ResponseWriter) WriteChunk(b []byte, end bool) error {
	if w.done.Load() {
		logger.Warn("Stream response already written, duplicate write, request ID: %d", w.id)
		return net.ErrClosed
	}

	logger.Debug("Writing stream response chunk, request ID: %d, data length: %d, end: %v", w.id, len(b), end)

	// 构建流式响应帧
	flags := protocol.FlagResponse | protocol.FlagStream
	if end {
		flags |= protocol.FlagStreamEnd
		w.done.Store(true)
		logger.Debug("Stream response ended, request ID: %d", w.id)
	}

	f := &protocol.Frame{Flags: byte(flags), ID: w.id, Type: w.route, Value: b}
	return w.conn.WriteFrame(f)
}

// SendMessage 发送消息到客户端（双向通信）
func (w *ResponseWriter) SendMessage(b []byte) error {
	if w.done.Load() {
		logger.Warn("Connection already closed, cannot send message, request ID: %d", w.id)
		return net.ErrClosed
	}

	logger.Debug("Sending message to client, request ID: %d, data length: %d", w.id, len(b))

	// 构建双向通信帧（不带响应标志，表示这是服务器主动发送的消息）
	f := &protocol.Frame{Flags: protocol.FlagStream, ID: w.id, Type: w.route, Value: b}
	return w.conn.WriteFrame(f)
}

// ReceiveMessage 接收客户端消息（双向通信）
func (w *ResponseWriter) ReceiveMessage() ([]byte, error) {
	select {
	case frame := <-w.incoming:
		if frame == nil {
			return nil, net.ErrClosed
		}
		return frame.Value, nil
	default:
		return nil, nil // 非阻塞，如果没有消息返回nil
	}
}

// ReceiveMessageBlocking 阻塞接收客户端消息（双向通信）
func (w *ResponseWriter) ReceiveMessageBlocking() ([]byte, error) {
	frame, ok := <-w.incoming
	if !ok || frame == nil {
		return nil, net.ErrClosed
	}
	return frame.Value, nil
}

// Close 关闭双向通信通道
func (w *ResponseWriter) Close() {
	w.done.Store(true)
	close(w.incoming)
}

// Context RPC请求上下文，包含请求信息和响应方法
type Context struct {
	Conn    *conn.Connection // 连接对象
	ID      uint64           // 请求ID
	Route   string           // 路由信息
	Payload []byte           // 请求数据
	H       Handler          // 处理函数
	w       *ResponseWriter  // 响应写入器
}

// NewContext 创建新的请求上下文
func NewContext(cn *conn.Connection, id uint64, route string, payload []byte, h Handler) *Context {
	return &Context{
		Conn:    cn,
		ID:      id,
		Route:   route,
		Payload: payload,
		H:       h,
		w:       NewResponseWriter(cn, id, []byte(route)),
	}
}

// Reply 发送同步响应
func (c *Context) Reply(b []byte) error {
	logger.Debug("Sending sync response, request ID: %d, route: %s", c.ID, c.Route)
	return c.w.Write(b)
}

// Stream 获取流式响应写入器
func (c *Context) Stream() *ResponseWriter {
	logger.Debug("Getting stream response writer, request ID: %d, route: %s", c.ID, c.Route)
	return c.w
}

// SendMessage 发送消息到客户端（双向通信）
func (c *Context) SendMessage(b []byte) error {
	logger.Debug("Sending message to client, request ID: %d, route: %s", c.ID, c.Route)
	return c.w.SendMessage(b)
}

// ReceiveMessage 接收客户端消息（双向通信，非阻塞）
func (c *Context) ReceiveMessage() ([]byte, error) {
	return c.w.ReceiveMessage()
}

// ReceiveMessageBlocking 接收客户端消息（双向通信，阻塞）
func (c *Context) ReceiveMessageBlocking() ([]byte, error) {
	return c.w.ReceiveMessageBlocking()
}

// HandleIncomingFrame 处理传入的帧（用于双向通信）
func (c *Context) HandleIncomingFrame(frame *protocol.Frame) {
	if c.w != nil && c.w.incoming != nil {
		select {
		case c.w.incoming <- frame:
			logger.Debug("Incoming frame queued for request ID: %d", c.ID)
		default:
			logger.Warn("Incoming frame dropped, channel full for request ID: %d", c.ID)
		}
	}
}

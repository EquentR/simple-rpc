// Package protocol 定义了TCP框架的通信协议
// 包含数据帧格式、编码解码逻辑等
package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
)

// Version 协议版本号
const Version byte = 1

// 帧标志位定义
const (
	FlagRequest   = 0x01 // 请求标志
	FlagResponse  = 0x02 // 响应标志
	FlagStream    = 0x04 // 流式数据标志
	FlagStreamEnd = 0x08 // 流式数据结束标志
	FlagError     = 0x10 // 错误标志
)

// Frame 数据帧结构
// 用于客户端和服务器之间的数据传输
type Frame struct {
	Flags byte   // 帧标志位，组合使用上述常量
	ID    uint64 // 请求ID，用于匹配请求和响应
	Type  []byte // 帧类型，通常表示路由或服务方法名
	Value []byte // 帧数据，具体的服务数据
}

// EncodeFrame 将Frame编码为字节数组
// 协议格式：
// [1字节版本][1字节标志][8字节ID][2字节类型长度][4字节数据长度][类型数据][值数据]
func EncodeFrame(f *Frame) ([]byte, error) {
	b := bytes.NewBuffer(nil)
	b.WriteByte(Version) // 写入版本号
	b.WriteByte(f.Flags) // 写入标志位

	// 写入请求ID（8字节，大端序）
	var id [8]byte
	binary.BigEndian.PutUint64(id[:], f.ID)
	b.Write(id[:])

	// 写入类型长度（2字节，大端序）
	var tl [2]byte
	binary.BigEndian.PutUint16(tl[:], uint16(len(f.Type)))
	b.Write(tl[:])

	// 写入数据长度（4字节，大端序）
	var vl [4]byte
	binary.BigEndian.PutUint32(vl[:], uint32(len(f.Value)))
	b.Write(vl[:])

	// 写入类型和数据
	b.Write(f.Type)
	b.Write(f.Value)

	return b.Bytes(), nil
}

// DecodeFrom 从Reader中解码Frame
// 按照EncodeFrame的格式进行解析
func DecodeFrom(r io.Reader) (*Frame, error) {
	// 读取固定头部（16字节）
	var h [16]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return nil, err
	}

	// 检查版本号
	if h[0] != Version {
		return nil, io.ErrUnexpectedEOF
	}

	// 解析标志位
	flags := h[1]

	// 解析请求ID
	id := binary.BigEndian.Uint64(h[2:10])

	// 解析类型长度
	tl := binary.BigEndian.Uint16(h[10:12])

	// 解析数据长度
	vl := binary.BigEndian.Uint32(h[12:16])

	// 读取类型数据
	tb := make([]byte, int(tl))
	if _, err := io.ReadFull(r, tb); err != nil {
		return nil, err
	}

	// 读取值数据
	vb := make([]byte, int(vl))
	if _, err := io.ReadFull(r, vb); err != nil {
		return nil, err
	}

	return &Frame{Flags: flags, ID: id, Type: tb, Value: vb}, nil
}

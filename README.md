# Simple RPC

一个高性能、轻量级的RPC框架，基于TCP构建并支持TLS，提供传统请求-响应模式和高级流式处理功能。

## 功能特性

- **TLS安全**: 内置TLS支持，确保通信安全
- **连接池管理**: 高效的连接池管理，支持自动健康检查
- **多种通信模式**:
  - 请求-响应: 传统RPC模式
  - 双向消息: 类似WebSocket的全双工通信
  - CallStream: 类似服务器发送事件(SSE)的流式处理
- **高性能**: 自定义协议，专为低延迟优化
- **并发处理**: 工作池模式处理多个请求
- **自动重连**: 连接池管理，支持自动恢复

## 架构概览

### 通信模式

#### 1. 请求-响应 (传统RPC)
类似HTTP请求-响应模式，但基于TCP和自定义协议：
```go
// 客户端
response, err := pool.Call("/health", payload, timeout)

// 服务端
func handleHealth(ctx *rpc.Context) {
    ctx.Reply([]byte(`{"status":"healthy"}`))
}
```

#### 2. 双向消息 (类似WebSocket)
全双工通信，客户端和服务端可以独立发送消息：

**客户端实现**:
```go
// 创建双向会话
session, err := pool.CreateBidirectionalSession("/stream", nil)

// 向服务端发送消息
err = session.Send([]byte("Hello, server!"))

// 接收服务端消息 (阻塞)
message, err := session.ReceiveBlocking()
```

**服务端实现**:
```go
func handleStream(ctx *rpc.Context) {
    // 接收客户端消息
    message, err := ctx.ReceiveMessageBlocking()
    
    // 向客户端发送消息
    err = ctx.SendMessage([]byte("Hello, client!"))
}
```

此模式提供：
- 实时双向通信
- 消息顺序保证
- 自动连接管理
- 适用于聊天应用、实时游戏、协作编辑

#### 3. CallStream (类似SSE)
服务端到客户端的流式传输，自动分块管理：

**客户端实现**:
```go
// 开始流式传输
streamChan, err := pool.CallStream("/stream/time", payload, timeout)

// 接收流消息
for data := range streamChan {
    fmt.Printf("接收到: %s\n", string(data))
}
```

**服务端实现**:
```go
func handleStreamTime(ctx *rpc.Context) {
    stream := ctx.Stream()
    
    // 持续发送数据流
    for i := 0; i < 10; i++ {
        message := fmt.Sprintf(`{"timestamp":%d}`, time.Now().Unix())
        isLast := i == 9
        
        err := stream.WriteChunk([]byte(message), isLast)
        if err != nil {
            return
        }
        
        time.Sleep(1 * time.Second)
    }
}
```

此模式提供：
- 自动分块流式传输
- 内置流量控制
- 高效内存使用
- 适用于实时数据流、进度更新、日志传输

## 快速开始

### 服务端设置

```go
package main

import (
    "crypto/tls"
    "github.com/EquentR/simple-rpc/server"
    "github.com/EquentR/simple-rpc/rpc"
)

func main() {
    // 创建TLS配置
    cert, tlsConfig, err := tlsutil.SelfSigned([]string{"localhost"})
    
    // 创建服务端
    srv := server.New("localhost:8443", tlsConfig, 8) // 8个工作协程
    
    // 注册路由
    srv.Mux.Handle("/health", handleHealth)
    srv.Mux.Handle("/echo", handleEcho)
    srv.Mux.Handle("/stream/time", handleStreamTime)
    
    // 启动服务端
    if err := srv.Serve(); err != nil {
        panic(err)
    }
}

func handleHealth(ctx *rpc.Context) {
    ctx.Reply([]byte(`{"status":"healthy"}`))
}
```

### 客户端设置

```go
package main

import (
    "crypto/tls"
    "github.com/EquentR/simple-rpc/client"
)

func main() {
    // 创建TLS配置
    tlsConfig := &tls.Config{InsecureSkipVerify: true}
    
    // 创建连接池
    pool := client.New("localhost:8443", tlsConfig, 3) // 3个连接
    defer pool.Close()
    
    // 简单请求-响应
    response, err := pool.Call("/health", []byte(""), 5*time.Second)
    
    // 流式传输
    streamChan, err := pool.CallStream("/stream/time", []byte(""), 15*time.Second)
    for data := range streamChan {
        fmt.Printf("流数据: %s\n", string(data))
    }
}
```

## 示例

`/cmd` 目录包含完整的示例：

### `/cmd/example/`
所有通信模式的完整演示：
- **服务端** (`server/main.go`): 实现健康检查、回显服务和时间流
- **客户端** (`client/main.go`): 演示请求-响应、流式传输和并发请求

### `/cmd/test/`
基础功能测试：
- **服务端** (`server/main.go`): 简单的双向消息实现
- **客户端** (`client/main.go`): 基础双向会话测试

## 协议详情

框架使用自定义二进制协议，专为以下目标优化：
- 最小化开销
- 快速解析
- 支持流式传输
- 内置压缩功能

### 帧结构
- **Flags**: 响应、流式传输、压缩的控制标志
- **ID**: 请求/响应关联
- **Type**: 路由信息
- **Value**: 负载数据

## 性能特性

- **延迟**: 本地连接亚毫秒级延迟
- **吞吐量**: 支持数千个并发连接
- **内存**: 高效连接池最小化内存使用
- **CPU**: 优化协议带来最小解析开销

## 使用场景

### 最佳适用场景
- **微服务通信**: 内部服务间RPC调用
- **实时应用**: 聊天、游戏、实时更新
- **数据流**: 日志聚合、指标收集
- **物联网通信**: 设备到服务器的消息传输

### 何时使用每种模式

| 模式 | 使用场景 | 示例 |
|---------|----------|---------|
| 请求-响应 | 传统API调用 | 用户认证、数据查询 |
| 双向消息 | 实时交互 | 聊天应用、多人游戏 |
| CallStream | 数据流 | 实时指标、进度更新、日志 |

## 高级功能

### 连接池管理
- 自动连接健康检查
- 跨连接负载均衡
- 故障自动重连
- 可配置池大小和超时

### 并发处理
- 工作池模式处理请求
- 可配置工作线程数
- 自动协程管理
- 请求排队和负载分发

### 错误处理
- 全面错误传播
- 连接故障恢复
- 超时管理
- 优雅降级

## 贡献

欢迎贡献！请随时提交问题和拉取请求。

## 许可证

本项目采用MIT许可证。
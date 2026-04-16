# Distributed Guide - 分布式部署指南

gomelo 支持多节点分布式部署，适合大规模游戏服务器架构。

## 架构概述

```
                        ┌─────────────┐
                        │   Master    │  ← 服务协调中心
                        │  (port 3040) │
                        └─────────────┘
                               │
    ┌──────────────────────────┼──────────────────────────┐
    │                          │                          │
┌───▼────┐              ┌──────▼──────┐              ┌──────▼──────┐
│connector│              │  connector  │              │  connector  │
│port 3010│              │  port 3011  │              │  port 3012  │  ← 前端层
└────┬────┘              └──────┬──────┘              └──────┬──────┘
     │                           │                          │
     └──────────────────────────┼──────────────────────────┘
                                │ RPC
                    ┌───────────┼───────────┐
                    │           │           │
              ┌─────▼─────┐┌───▼────┐┌─────▼─────┐
              │    chat    ││  game  ││   auth    │  ← 后端层
              │  port 3020 ││port3030││ port 3040 │
              └───────────┘└────────┘└───────────┘
```

## 核心组件

### Master Server

Master 负责协调所有服务器：

```go
master := master.New(":3040")
master.Start()
```

### Registry

服务注册中心，用于服务发现：

```go
reg := registry.New()
```

### Selector

负载均衡选择器：

```go
sel := selector.NewRandomSelector()
// 或一致性哈希
sel := selector.NewConsistentHashSelector(100, nil)
```

## 配置多服务器

### servers.json

```json
{
  "development": {
    "connector": [
      {"id": "connector-1", "host": "127.0.0.1", "port": 3010}
    ],
    "chat": [
      {"id": "chat-1", "host": "127.0.0.1", "port": 3020}
    ]
  },
  "production": {
    "connector": [
      {"id": "connector-1", "host": "10.0.0.1", "port": 3010},
      {"id": "connector-2", "host": "10.0.0.2", "port": 3010}
    ],
    "chat": [
      {"id": "chat-1", "host": "10.0.1.1", "port": 3020},
      {"id": "chat-2", "host": "10.0.1.2", "port": 3020}
    ],
    "game": [
      {"id": "game-1", "host": "10.0.2.1", "port": 3030}
    ]
  }
}
```

## 前端服务器 (Connector)

处理客户端连接：

```go
app := gomelo.NewApp(
    gomelo.WithPort(3010),
    gomelo.WithServerID("connector-1"),
    gomelo.WithMasterAddr("127.0.0.1:3040"),
)

app.Configure("connector", "connector")(func(s *gomelo.Server) {
    s.SetFrontend(true)
    s.SetPort(3010)
})

// 注册到 Master
app.On("connector.entry", handleEntry)

// 转发消息到后端
app.On("chat.send", handleForwardToChat)
```

## 后端服务器 (Backend)

处理业务逻辑：

```go
app := gomelo.NewApp(
    gomelo.WithPort(3020),
    gomelo.WithServerID("chat-1"),
    gomelo.WithMasterAddr("127.0.0.1:3040"),
)

app.Configure("chat", "chat")(func(s *gomelo.Server) {
    s.SetFrontend(false)
    s.SetPort(3020)
})

app.On("chat.send", handleChatSend)
```

## RPC 调用

### 创建 RPC Client

```go
client := rpc.NewClient(&rpc.ClientOptions{
    Host:    "127.0.0.1",
    Port:    3020,
    MaxConns: 5,
    Timeout:  5 * time.Second,
})
```

### 使用连接池

```go
pool := rpc.NewClientPool("127.0.0.1:3020", 10, 1, 5*time.Second)
client, err := pool.GetClient()
if err != nil {
    log.Fatal(err)
}
defer pool.Close()
```

### 调用示例

```go
var reply struct {
    Code int `json:"code"`
    Msg  string `json:"msg"`
}

err := client.Invoke("chat", "Send", &req, &reply)
if err != nil {
    log.Printf("RPC error: %v", err)
}
```

### Notify (无响应调用)

```go
err := client.Notify("chat", "Broadcast", map[string]any{
    "roomId": "room-1",
    "msg":    "hello",
})
```

## 消息转发

### Forwarder

消息转发器自动处理跨服务器通信：

```go
forward := forward.NewForwarder(app, selector)

func handleForward(ctx *gomelo.Context) {
    var req struct {
        Target string `json:"target"`
        Route  string `json:"route"`
        Data   any    `json:"data"`
    }
    ctx.Bind(&req)

    // 选择目标服务器
    servers := registry.GetServersByType(req.Target)
    if len(servers) == 0 {
        ctx.ResponseError(errors.New("no server available"))
        return
    }

    server := selector.Select(servers)
    forward.Forward(ctx.Session(), ctx.Message(), server)
}
```

## 服务注册与发现

### 注册服务

```go
// 连接 Master
master := master.New("127.0.0.1:3040")

// 注册当前服务器
master.AddServer(&master.ServerInfo{
    ID:         "connector-1",
    ServerType: "connector",
    Host:       "127.0.0.1",
    Port:       3010,
    Frontend:   true,
})
```

### 订阅变更

```go
registry.Watch(func(event string, servers []*registry.ServerInfo) {
    log.Printf("Event: %s, Servers: %d", event, len(servers))
    for _, s := range servers {
        log.Printf("  - %s: %s:%d", s.ID, s.Host, s.Port)
    }
})
```

## 负载均衡策略

### 随机选择

```go
sel := selector.NewRandomSelector()
server := sel.Select(keys...)
```

### 轮询

```go
sel := selector.NewRoundRobinSelector()
server := sel.Select(keys...)
```

### 一致性哈希

适合有状态服务，减少数据迁移：

```go
sel := selector.NewConsistentHashSelector(100, nil) // 100 个虚拟节点
sel.AddServer(serverInfo1)
sel.AddServer(serverInfo2)
server := sel.Select(uid) // 同一 UID 路由到同一服务器
```

## 广播

### 全服广播

```go
broadcast := broadcast.NewBroadcast("announcement")
broadcast.Broadcast("server.announcement", map[string]any{
    "msg": "Server maintenance in 5 minutes",
})
```

### 指定用户广播

```go
broadcast.BroadcastTo([]string{"user-1", "user-2", "user-3"}, "chat.message", map[string]any{
    "from": "admin",
    "msg":  "hello",
})
```

## 完整示例

### 启动 Master

```go
// master/main.go
package main

import (
    "log"
    "gomelo/master"
)

func main() {
    m := master.New(":3040")
    log.Println("Master starting on :3040")
    if err := m.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### 启动 Connector

```go
// connector/main.go
package main

import (
    "log"
    "gomelo"
    "gomelo/master"
)

func main() {
    masterServer, _ := master.New("127.0.0.1:3040")

    app := gomelo.NewApp(
        gomelo.WithPort(3010),
        gomelo.WithServerID("connector-1"),
    )

    app.Configure("connector", "connector")(func(s *gomelo.Server) {
        s.SetFrontend(true)
        s.SetPort(3010)
    })

    app.On("connector.entry", handleEntry)

    app.Start(func(err error) {
        if err != nil {
            log.Fatal(err)
        }
        masterServer.AddServer(&master.ServerInfo{
            ID:         "connector-1",
            ServerType: "connector",
            Host:       "127.0.0.1",
            Port:       3010,
        })
    })
}
```

### 启动 Chat Server

```go
// chat/main.go
package main

import (
    "log"
    "gomelo"
    "gomelo/master"
    "gomelo/rpc"
)

func main() {
    masterServer, _ := master.New("127.0.0.1:3040")

    app := gomelo.NewApp(
        gomelo.WithPort(3020),
        gomelo.WithServerID("chat-1"),
    )

    app.Configure("chat", "chat")(func(s *gomelo.Server) {
        s.SetFrontend(false)
        s.SetPort(3020)
    })

    // 注册 RPC Handler
    rpcServer := rpc.NewServer(":3030")
    rpcServer.Register(&ChatRPC{})
    go rpcServer.Start()

    app.On("chat.send", handleChatSend)

    app.Start(func(err error) {
        if err != nil {
            log.Fatal(err)
        }
        masterServer.AddServer(&master.ServerInfo{
            ID:         "chat-1",
            ServerType: "chat",
            Host:       "127.0.0.1",
            Port:       3020,
        })
    })
}

type ChatRPC struct{}

func (s *ChatRPC) Send(ctx context.Context, args struct {
    From string `json:"from"`
    To   string `json:"to"`
    Msg  string `json:"msg"`
}) (any, error) {
    log.Printf("Chat: %s -> %s: %s", args.From, args.To, args.Msg)
    return map[string]any{"code": 0}, nil
}
```

## 健康检查

心跳检测自动移除宕机服务器：

```go
master.OnStateChange(func(id string, oldState, newState int) {
    log.Printf("Server %s state changed: %d -> %d", id, oldState, newState)
    if newState == 3 { // 超时状态
        log.Printf("Server %s is down", id)
    }
})
```

## 最佳实践

1. **前后端分离** - Connector 只做连接管理，业务逻辑放到后端
2. **使用连接池** - 避免频繁创建 RPC 连接
3. **合理选择负载均衡** - 无状态用轮询/随机，有状态用一致性哈希
4. **监控服务器状态** - 订阅 OnStateChange 及时响应服务器下线
5. **优雅关闭** - 关闭前先从 Master 注销
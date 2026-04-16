# gomelo

高性能分布式游戏服务端框架，采用 Go 语言实现，源自 Node.js Pomelo 架构设计。

## 特性

- **分布式架构** - 支持多节点部署，前端/后端分离
- **高性能 RPC** - 连接池复用，异步消息转发，支持双向追踪
- **类型安全** - 强类型 Filter 接口和 Handler 签名
- **服务注册发现** - Master 协调 + Registry 双模式
- **负载均衡** - 轮询、一致性哈希、加权随机多种策略
- **批量广播** - 异步批量推送，支持按 UID/ID 分组
- **生产级功能** - 熔断器、限流、指标采集、健康检查、链路追踪
- **优雅关闭** - 超时控制，确保任务完成
- **配置热更新** - 文件监控自动 reload

## 环境要求

- Go 1.21+

## 快速开始

### 1. 安装 CLI

```bash
git clone https://github.com/gomelo/gomelo.git
cd gomelo
go build -o bin/gomelo ./cmd/cli
```

### 2. 全局安装（可选）

```bash
# Linux/Mac
sudo cp bin/gomelo /usr/local/bin/

# Windows (PowerShell 管理员)
Copy bin\gomelo.exe C:\Windows\System32\
```

### 3. 初始化项目

```bash
gomelo init mygame
cd mygame
go mod tidy
```

### 4. 启动项目

```bash
go run .
```

## 项目结构

```
mygame/
├── main.go
├── go.mod
├── config/
│   ├── servers.json     # 多服务器配置
│   └── log.json         # 日志配置
├── servers/             # 服务器定义
│   ├── connector/
│   │   ├── handler/
│   │   ├── remote/
│   │   ├── filter/
│   │   └── cron/
│   ├── gate/
│   │   ├── handler/
│   │   ├── remote/
│   │   └── filter/
│   ├── chat/
│   │   ├── handler/
│   │   ├── remote/
│   │   └── filter/
│   └── game/
│       ├── handler/
│       ├── remote/
│       └── filter/
├── components/
└── logs/
```

## 示例代码

### 最小入口 (main.go)

```go
package main

import (
	"log"
	"gomelo"
)

func main() {
	app := gomelo.NewApp(
		gomelo.WithHost("0.0.0.0"),
		gomelo.WithPort(3010),
		gomelo.WithServerID("connector-1"),
	)

	app.Configure("connector", "connector")(func(s *gomelo.Server) {
		s.SetFrontend(true)
		s.SetPort(3010)
	})

	app.On("connector.entryHandler.entry", handleEntry)

	log.Println("Starting server...")
	app.Start(func(err error) {
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Server started!")
	})

	app.Wait()
}

func handleEntry(ctx *gomelo.Context) {
	var req struct {
		Name string `json:"name"`
	}
	ctx.Bind(&req)

	ctx.Session().Set("uid", "user-"+strconv.FormatUint(ctx.Session().ID(), 10))

	ctx.Response(map[string]any{
		"code": 0,
		"msg":  "ok",
		"data": map[string]any{
			"uid": ctx.Session().Get("uid"),
		},
	})
}
```

### Session 管理

```go
func handleEntry(ctx *gomelo.Context) {
	session := ctx.Session()

	// 设置用户绑定
	session.Bind("user-123")

	// 存储数据
	session.Set("level", 10)
	session.Set("name", "player")

	// 获取数据
	uid := session.UID()          // "user-123"
	level := session.Get("level") // 10

	// 踢出玩家
	session.Close()
}
```

### 消息广播

```go
func handleChatSend(ctx *gomelo.Context) {
	var req struct {
		Msg    string `json:"msg"`
		RoomID string `json:"roomId"`
	}
	ctx.Bind(&req)

	uid := ctx.Session().Get("uid")

	// 广播到指定房间
	broadcast := gomelo.NewBroadcast("room." + req.RoomID)
	broadcast.BroadcastTo([]string{"user-1", "user-2"}, "chat.message", map[string]any{
		"uid":  uid,
		"msg":  req.Msg,
		"time": time.Now().Unix(),
	})
}
```

### RPC 调用

```go
func handleForwardToChat(ctx *gomelo.Context) {
	var req struct {
		Target string `json:"target"`
		Msg    string `json:"msg"`
	}
	ctx.Bind(&req)

	// 转发到其他服务器
	forward := gomelo.NewForwarder(app, selector)
	forward.Forward(ctx.Session(), ctx.Message(), serverInfo)
}
```

### 配置热更新

```go
app := gomelo.NewApp()

// 启用配置热更新
watcher, _ := config.NewWatcher("config.json")
watcher.Watch(func(cfg *config.Config) {
	log.Printf("Config reloaded: %+v", cfg)
	app.Set("config", cfg)
})
```

## 分布式部署架构

```
                        ┌─────────────┐
                        │   Master    │  ← 服务协调中心
                        └─────────────┘
                               │
    ┌──────────────────────────┼──────────────────────────┐
    │                          │                          │
┌───▼────┐              ┌──────▼──────┐              ┌──────▼──────┐
│connector│              │  connector  │              │  connector  │  ← 前端层
│(Frontend)│             │  (Frontend) │              │  (Frontend) │
└────┬────┘              └──────┬───────┘              └──────┬──────┘
     │                          │                              │
     └──────────────────────────┼──────────────────────────────┘
                                 │ RPC
                    ┌────────────┼────────────┐
                    │            │            │
              ┌──────▼──────┐┌────▼────┐┌────▼────┐
              │    chat     ││   game  ││   auth  │  ← 后端层
              └─────────────┘└─────────┘└─────────┘
```

## CLI 命令

| 命令 | 说明 |
|------|------|
| `gomelo init <name>` | 初始化新项目 |
| `gomelo add <type>` | 添加服务器类型 (connector/chat/gate/auth/game/match) |
| `gomelo start` | 启动应用 |
| `gomelo build` | 构建应用 |
| `gomelo clean` | 清理构建产物 |
| `gomelo -v` | 查看版本 |
| `gomelo -h` | 查看帮助 |

## 自动路由注册

使用 codegen 自动扫描服务器代码并生成注册代码：

```bash
go run ./cmd/codegen ./servers
```

这会扫描 `servers/{serverType}/handler/` 和 `servers/{serverType}/remote/` 目录，自动注册所有 Handler 和 Remote 方法。

详细文档：[Handler-Guide.md](docs/Handler-Guide.md)

## 核心 API

### App

| 方法 | 说明 |
|------|------|
| `NewApp(opts...)` | 创建应用实例 |
| `WithHost(host)` | 设置监听地址 |
| `WithPort(port)` | 设置监听端口 |
| `WithServerID(id)` | 设置服务器 ID |
| `WithMasterAddr(addr)` | 设置 Master 地址 |
| `Configure(env, serverType)` | 配置服务器类型 |
| `On(route, handler)` | 注册路由处理器 |
| `Before(filter)` | 注册前置过滤器 |
| `After(filter)` | 注册后置过滤器 |
| `Start(cb)` | 启动应用 |
| `Stop()` | 停止应用 |
| `Wait()` | 阻塞等待信号 |

### Context

| 方法 | 说明 |
|------|------|
| `Session()` | 获取当前 Session |
| `Message()` | 获取当前 Message |
| `Bind(v)` | 解析请求数据 |
| `Response(v)` | 发送响应 |
| `ResponseError(err)` | 发送错误响应 |
| `Next()` | 调用下一个处理器 |

### Session

| 方法 | 说明 |
|------|------|
| `ID()` | 获取会话 ID |
| `UID()` | 获取绑定用户 ID |
| `Bind(uid)` | 绑定用户 ID |
| `Set(key, val)` | 存储数据 |
| `Get(key)` | 获取数据 |
| `Remove(key)` | 删除数据 |
| `Push(key, val, cb)` | 推送数据到客户端 |
| `Close()` | 关闭会话 |
| `OnClose(handler)` | 注册关闭回调 |

### Server

| 方法 | 说明 |
|------|------|
| `SetFrontend(v)` | 设置是否为前端服务器 |
| `SetPort(port)` | 设置端口 |
| `SetHost(host)` | 设置地址 |
| `SetServerType(t)` | 设置服务器类型 |
| `OnConnection(fn)` | 连接回调 |
| `OnMessage(fn)` | 消息回调 |
| `OnClose(fn)` | 关闭回调 |

## 目录结构

```
gomelo/
├── gomelo.go           # 入口，导出所有公共 API
├── lib/                 # 核心库
│   ├── app.go          # 应用主体
│   ├── session.go      # 会话管理
│   ├── context.go      # 请求上下文
│   ├── router.go       # 路由
│   ├── pipeline.go    # 中间件管道
│   ├── event.go        # 事件发射器
│   ├── error.go        # 错误定义
│   ├── lifecycle.go    # 生命周期接口
│   ├── circuitbreaker.go # 熔断器
│   ├── ratelimit.go    # 限流
│   ├── metrics.go      # 指标
│   ├── health.go       # 健康检查
│   ├── tracing.go      # 链路追踪
│   └── shutdown.go     # 优雅关闭
├── rpc/                 # RPC 系统
│   ├── client.go       # RPC 客户端 + 连接池
│   └── server.go       # RPC 服务端
├── connector/           # 网络连接器
├── master/             # Master 服务器
├── registry/           # 服务注册中心
├── server_registry/     # 服务器注册表
├── selector/           # 负载均衡选择器
├── forward/            # 消息转发
├── broadcast/           # 广播服务
├── pool/               # 连接池 + WorkerPool
├── scheduler/          # 定时任务调度器
├── loader/             # Handler/Remote 加载器
├── config/             # 配置系统
├── codec/              # 消息编解码
├── filter/             # Filter 接口
├── route/              # 路由压缩
├── logger/             # 日志
├── plugin/             # 插件系统
├── component/          # 组件接口
├── websocket/          # WebSocket 支持
└── cmd/                # 命令行工具
    ├── cli/            # gomelo CLI
    ├── demo/           # 示例
    └── codegen/        # 代码生成器
```

## 与 Node.js Pomelo 对比

| 功能 | Node.js Pomelo | gomelo |
|------|---------------|--------|
| 安装 | `npm install -g pomelo` | `go build ./cmd/cli` |
| 初始化 | `pomelo init mygame` | `gomelo init mygame` |
| 启动 | `node start.js` | `go run .` |
| 入口文件 | `start.js` | `main.go` |
| Handler 签名 | `function(session, msg, next)` | `func(ctx *Context)` |
| Filter 接口 | `before/after filter` | `Before/After filter` |
| RPC | `pomelo.rpc.invoke` | `client.Invoke(service, method, args, reply)` |

## 性能指标

- RPC 连接池复用率: >90%
- 消息转发延迟: <1ms
- 单节点支持连接: 10000+
- 支持 Goroutine 池化，避免无限创建

## 文档

- [English Version](README_en.md)
- [Handler Guide](docs/Handler-Guide.md)
- [Getting Started](docs/Getting-Started.md)
- [Session Guide](docs/Session-Guide.md)
- [Distributed Guide](docs/Distributed-Guide.md)
- [API Reference](docs/API.md)

## 许可证

MIT
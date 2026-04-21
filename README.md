**[English](README_en.md)** | 简体中文

---

# gomelo

高性能分布式游戏服务端框架，采用 Go 语言实现，源自 Node.js Pomelo 架构设计。

## 特性

- **分布式架构** - 支持多节点部署，前端/后端分离
- **高性能 RPC** - 连接池复用，异步消息转发，支持双向追踪
- **类型安全** - 强类型 Filter 接口和 Handler 签名
- **服务注册发现** - Master 协调 + Registry 双模式，支持断线重连
- **负载均衡** - 轮询、一致性哈希、加权随机多种策略
- **批量广播** - 异步批量推送，支持按 UID/ID 分组
- **生产级功能** - 熔断器、限流、指标采集、健康检查
- **优雅关闭** - 超时控制，确保任务完成
- **配置热更新** - 文件监控自动 reload
- **多语言客户端** - JavaScript、GDScript、C# 完整支持二进制协议

## 环境要求

- Go 1.21+

## 快速开始

### 1. 安装 CLI

```bash
# 方式一：go install（推荐，Go 1.16+）
go install github.com/chuhongliang/gomelo/cmd/gomelo@latest

# 方式二：手动编译
git clone https://github.com/chuhongliang/gomelo.git
cd gomelo
go build -o bin/gomelo ./cmd/gomelo
```

### 2. 初始化项目

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
game-project/
├── game-server/           # 游戏服务器
│   ├── main.go
│   ├── go.mod
│   ├── config/
│   │   ├── servers.json
│   │   ├── log.json
│   │   └── master.json
│   ├── servers/          # 服务器定义
│   │   ├── connector/
│   │   ├── gate/
│   │   ├── chat/
│   │   └── game/
│   ├── components/      # 共享组件
│   ├── cmd/admin/        # 监控管理后台
│   └── logs/            # 日志目录
├── web-server/           # 前端静态资源
│   └── public/
│       ├── index.html
│       └── js/client.js
└──```

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

	// 启动时自动注册 servers/connector/handler 下的所有 Handler
	log.Println("Starting server...")
	app.Start(func(err error) {
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Server started!")
	})

	app.Wait()
}
```

### Handler 示例

```go
// servers/connector/handler/entry.go
package handler

type EntryHandler struct{}

func (h *EntryHandler) Entry(ctx *gomelo.Context) {
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

自动生成的路由：`connector.entryHandler.entry`

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
│   ├── event.go        # 事件发射器
│   ├── metrics.go      # 指标采集
│   ├── health.go       # 健康检查
│   └── shutdown.go     # 优雅关闭
├── rpc/                 # RPC 系统
│   ├── client.go       # RPC 客户端 + 连接池
│   └── server.go       # RPC 服务端
├── connector/           # 网络连接器
├── master/             # Master 服务器
├── registry/           # 服务注册中心
├── selector/           # 负载均衡选择器
├── forward/            # 消息转发
├── broadcast/           # 广播服务
├── pool/               # 连接池 + WorkerPool
├── loader/             # Handler/Remote 加载器
├── codec/              # 消息编解码（JSON/Protobuf）
├── proto/              # protobuf 消息定义
├── client/             # 客户端 SDK
│   ├── js/             # JavaScript 客户端
│   ├── godot/          # Godot GDScript 客户端
│   └── unity/          # Unity C# 客户端
└── cmd/                # 命令行工具
    ├── cli/            # gomelo CLI
    ├── demo/           # 示例
    └── codegen/        # 代码生成器
```

## 客户端 SDK

### JavaScript 客户端

```javascript
import { GomeloClient, MessageType } from './client/js/client.js';

const client = new GomeloClient({ host: 'localhost', port: 3010 });
await client.connect();

// 注册路由（可选）
client.registerRoute('connector.entry', 1);

// request-response
const res = await client.request('connector.entry', { name: 'Alice' });

// notify（无响应）
client.notify('player.move', { position: { x: 1, y: 2, z: 3 } });

// 事件监听
client.on('onChat', (msg) => console.log('Chat:', msg));
```

### Godot GDScript 客户端

```gdscript
var client: GomeloClient

func _ready():
    client = GomeloClient.new()
    add_child(client)
    client.connect_to_server("localhost", 3010)
    client.connect("connected", Callable(self, "_on_connected"))

func _on_connected():
    var seq = client.request("player.entry", {"name": "Player1"})
    client.on("onChat", func(body): print("Chat: ", body))
    client.notify("player.move", {"position": {"x": 1, "y": 2, "z": 3}})
```

### Unity C# 客户端

```csharp
using Gomelo;

public class GameManager : MonoBehaviour
{
    private GomeloClient _client;

    void Start()
    {
        _client = gameObject.AddComponent<GomeloClient>();
        _client.OnConnected += OnConnected;
        _client.OnError += (msg) => Debug.LogError("Error: " + msg);
        _client.Connect("localhost", 3010);

        // 注册路由
        _client.RegisterRoute("player.entry", 1);

        // 事件监听
        _client.On("onChat", (body) => Debug.Log("Chat: " + body));
    }

    void OnConnected()
    {
        // request-response
        _client.Request("player.entry", new { name = "Player1" },
            (body) => Debug.Log("Success: " + body),
            (err) => Debug.LogError("Error: " + err));

        // notify（无响应）
        _client.Notify("player.move", new { position = new { x = 1, y = 2, z = 3 } });
    }
}
```

详细文档：[Handler-Guide.md](docs/Handler-Guide.md)

## 与 Node.js Pomelo 对比

| 功能 | Node.js Pomelo | gomelo |
|------|---------------|--------|
| 安装 | `npm install -g pomelo` | `go build ./cmd/gomelo` |
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

- [Handler 指南](docs/Handler-Guide.md)
- [快速开始](docs/Getting-Started.md)
- [Session 管理](docs/Session-Guide.md)
- [分布式部署](docs/Distributed-Guide.md)
- [API 参考](docs/API.md)

## 许可证

MIT
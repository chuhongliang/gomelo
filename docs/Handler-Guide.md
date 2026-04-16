# Handler Guide - 处理器指南

Handlers 是处理客户端请求的核心组件。

## 基本概念

Handler 是一个接收 `*Context` 参数的函数：

```go
func handlerName(ctx *gomelo.Context)
```

## 注册 Handler

通过 `app.On(route, handler)` 注册：

```go
app.On("connector.entry", handleEntry)
app.On("chat.send", handleChatSend)
app.On("game.battle", handleBattle)
```

## Context 常用方法

### 获取请求数据 - Bind

```go
func handleEntry(ctx *gomelo.Context) {
    var req struct {
        Token string `json:"token"`
        Name  string `json:"name"`
    }
    if err := ctx.Bind(&req); err != nil {
        ctx.ResponseError(gomelo.ErrInvalidRoute)
        return
    }

    log.Printf("Received: token=%s, name=%s", req.Token, req.Name)
}
```

### 发送响应 - Response

```go
ctx.Response(map[string]any{
    "code": 0,
    "msg":  "ok",
    "data": map[string]any{
        "uid": ctx.Session().UID(),
    },
})
```

### 获取 Session

```go
session := ctx.Session()
session.Set("level", 10)
uid := session.UID()
```

### 获取 Message

```go
msg := ctx.Message()
log.Printf("Route: %s, Body: %v", msg.Route, msg.Body)
```

## 结构化 Handler

可以将多个 handler 组织到一个结构体中：

```go
type ConnectorHandler struct {
    app *gomelo.App
}

func (h *ConnectorHandler) Entry(ctx *gomelo.Context) {
    var req struct {
        Token string `json:"token"`
    }
    ctx.Bind(&req)

    uid := "user-" + ctx.Session().ID()
    ctx.Session().Bind(uid)

    ctx.Response(map[string]any{
        "code": 0,
        "msg":  "ok",
        "data": map[string]any{"uid": uid},
    })
}

func (h *ConnectorHandler) Heartbeat(ctx *gomelo.Context) {
    ctx.Response(map[string]any{"code": 0, "msg": "pong"})
}
```

注册：

```go
handler := &ConnectorHandler{app: app}
app.On("connector.entry", handler.Entry)
app.On("connector.heartbeat", handler.Heartbeat)
```

## 错误处理

### 使用预定义错误

```go
ctx.ResponseError(gomelo.ErrUnauthorized)
ctx.ResponseError(gomelo.ErrInvalidRoute)
ctx.ResponseError(gomelo.ErrTimeout)
```

### 自定义错误

```go
ctx.ResponseError(fmt.Errorf("custom error: %w", someErr))
```

## 前置处理 - Middleware

使用 `app.Before()` 添加前置处理：

```go
app.Before(handleAuth)
app.Before(handleLog)
```

Middleware 函数签名：

```go
func handleAuth(ctx *gomelo.Context) bool {
    token := ctx.Session().Get("token")
    if token == nil {
        ctx.Response(map[string]any{"code": 401, "msg": "unauthorized"})
        return false
    }
    return true
}
```

## 后置处理 - AfterFilter

```go
app.After(handleAfter)
```

```go
func handleAfter(ctx *gomelo.Context) {
    log.Printf("Request completed: %s", ctx.Route)
}
```

## 完整示例

```go
package main

import (
	"log"
	"strconv"
	"gomelo"
)

func main() {
	app := gomelo.NewApp(
		gomelo.WithPort(3010),
		gomelo.WithServerID("connector-1"),
	)

	app.Configure("connector", "connector")(func(s *gomelo.Server) {
		s.SetFrontend(true)
		s.SetPort(3010)
		s.OnConnection(func(session *gomelo.Session) {
			log.Printf("Client connected: %d", session.ID())
		})
	})

	app.Before(authMiddleware)

	app.On("connector.entry", handleEntry)
	app.On("connector.heartbeat", handleHeartbeat)
	app.On("chat.send", handleChatSend)

	app.Start(func(err error) {
		if err != nil {
			log.Fatal(err)
		}
	})
	app.Wait()
}

func authMiddleware(ctx *gomelo.Context) bool {
	token := ctx.Session().Get("token")
	if token == nil {
		ctx.Response(map[string]any{"code": 401, "msg": "no token"})
		return false
	}
	return true
}

func handleEntry(ctx *gomelo.Context) {
	var req struct {
		Name string `json:"name"`
	}
	ctx.Bind(&req)

	uid := "user-" + strconv.FormatUint(ctx.Session().ID(), 10)
	ctx.Session().Bind(uid)
	ctx.Session().Set("name", req.Name)

	ctx.Response(map[string]any{
		"code": 0,
		"msg":  "ok",
		"data": map[string]any{
			"uid":  uid,
			"name": req.Name,
		},
	})
}

func handleHeartbeat(ctx *gomelo.Context) {
	ctx.Response(map[string]any{"code": 0, "msg": "pong"})
}

func handleChatSend(ctx *gomelo.Context) {
	var req struct {
		Msg string `json:"msg"`
	}
	ctx.Bind(&req)

	uid := ctx.Session().Get("uid")
	log.Printf("%s: %s", uid, req.Msg)

	ctx.Response(map[string]any{"code": 0, "msg": "ok"})
}
```

## 路由约定

建议使用 `serverType.handler` 格式：

| 路由 | 说明 |
|------|------|
| `connector.entry` | 连接器入口 |
| `connector.heartbeat` | 心跳检测 |
| `chat.send` | 聊天发送 |
| `chat.join` | 加入房间 |
| `game.start` | 游戏开始 |
| `game.move` | 游戏移动 |

## 自动注册（推荐）

使用 codegen 自动扫描和注册 Handler，无需手动调用 `app.On()`。

### 目录结构

```
game-server/app/servers/
  {serverType}/
    handler/      # 处理器目录
      entry.go    # 自动扫描此目录
    remote/       # RPC 目录
    filter/       # 过滤器目录
    cron/         # 定时任务目录
```

### Handler 命名规范

- 类型名以 `Handler` 或 `handler` 结尾
- 方法接收 `*lib.Context` 参数

```go
// game-server/app/servers/connector/handler/entry.go
package handler

import "gomelo/lib"

type EntryHandler struct {
    app *lib.App
}

func (h *EntryHandler) Init(app *lib.App) { h.app = app }

func (h *EntryHandler) Entry(ctx *lib.Context) {
    var req struct {
        Name string `json:"name"`
    }
    ctx.Bind(&req)
    ctx.Response(map[string]any{"msg": "hello " + req.Name})
}
```

### 运行代码生成

```bash
go run ./cmd/codegen ./game-server/app/servers
```

生成 `servers_gen.go`，自动注册所有 Handler 和 Remote。

### 生成的文件示例

```go
// servers_gen.go (自动生成)
func init() {
    l := loader.GlobalLoader()
    if l == nil { return }

    hEntryHandler := &handler.EntryHandler{}
    vEntryHandler := loader.ReflectValueOf(hEntryHandler)
    tEntryHandler := vEntryHandler.Type()
    for i := 0; i < tEntryHandler.NumMethod(); i++ {
        m := tEntryHandler.Method(i)
        if loader.IsHandlerMethod(m) {
            route := loader.BuildRoute("connector", tEntryHandler.Elem().Name(), m.Name)
            l.RegisterHandlerMethod("connector", route, hEntryHandler, m)
        }
    }
}
```

### 路由规则

| Handler 方法 | 自动生成路由 |
|-------------|-------------|
| `EntryHandler.Entry` | `connector.entryHandler.entry` |
| `ChatHandler.Send` | `connector.chatHandler.send` |

可结合 `app.Route()` 自定义路由前缀。

### 初始化回调

如果 Handler 实现了 `Init(app *lib.App)` 方法，会在注册后自动调用：

```go
func (h *EntryHandler) Init(app *lib.App) {
    h.app = app
}
```

| 路由 | 说明 |
|------|------|
| `connector.entry` | 连接器入口 |
| `connector.heartbeat` | 心跳检测 |
| `chat.send` | 聊天发送 |
| `chat.join` | 加入房间 |
| `game.start` | 游戏开始 |
| `game.move` | 游戏移动 |
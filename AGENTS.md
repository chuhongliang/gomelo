# gomelo 开发指南

## 关键约束

- **入口文件**：`gomelo.go`，导出所有公共 API
- **lib/ 单一 package**：避免循环依赖，router.go 只依赖 Context 接口
- **依赖单向**：lib → rpc → registry → selector，无反向依赖
- **Go 版本**：1.21+
- **无测试文件**：项目中暂无单元测试

## 开发命令

```bash
# 构建 CLI 工具
go build -o bin/gomelo ./cmd/cli

# 运行 demo
go run ./cmd/demo

# 构建所有包
go build ./...

# 代码生成（扫描 servers 目录生成注册代码）
go run ./cmd/codegen ./game-server/app/servers
```

## 模块概览

| 目录 | 用途 |
|------|------|
| `lib/` | 核心：App, Session, Message, Router, Event |
| `rpc/` | RPC 客户端 + 连接池 |
| `registry/` | 服务注册中心 |
| `selector/` | 负载均衡选择器 |
| `connector/` | 网络连接器 |
| `broadcast/` | 批量广播 |
| `forward/` | 消息转发 |
| `master/` | Master 协调 |
| `loader/` | 服务器代码加载器（Handler/Remote） |
| `codec/` | 消息编解码（JSON/Protobuf） |
| `proto/` | protobuf 消息定义（protoc 生成） |
| `client/` | 客户端 SDK（JS, Godot） |
| `cmd/codegen/` | 代码生成器 |

## Pomelo 目录结构

遵循 Pomelo 约定，服务器代码组织在 `servers/` 目录下：

```
servers/{serverType}/
  handler/        # 处理客户端请求
  remote/         # 处理 RPC 调用
  filter/        # 过滤器
  cron/          # 定时任务
```

### 代码生成

运行 codegen 扫描目录并生成注册代码：

```bash
go run ./cmd/codegen ./servers
```

生成 `servers_gen.go` 文件，自动注册所有 Handler 和 Remote。

### Handler（处理客户端请求）

```go
// servers/connector/handler/entry.go
package handler

import (
    "gomelo/lib"
)

type EntryHandler struct {
    app *lib.App
}

func (h *EntryHandler) Init(app *lib.App) { h.app = app }

func (h *EntryHandler) Entry(ctx *lib.Context) {
    var req struct { Name string `json:"name"` }
    ctx.Bind(&req)
    ctx.Response(map[string]any{"msg": "hello " + req.Name})
}
```

命名规范：
- 类型名以 `Handler` 结尾
- 方法接收 `*lib.Context` 参数

### Remote（处理 RPC 调用）

```go
// servers/connector/remote/connector.go
package remote

import (
    "context"
    "gomelo/lib"
)

type ConnectorRemote struct {
    app *lib.App
}

func (r *ConnectorRemote) Init(app *lib.App) { r.app = app }

func (r *ConnectorRemote) AddUser(ctx context.Context, args struct {
    UserID string `json:"userId"`
}) (any, error) {
    return map[string]any{"code": 0, "user": args.UserID}, nil
}
```

命名规范：
- 类型名以 `Remote` 结尾
- 方法接收 `context.Context` 和 `args` 参数，返回 `(any, error)`

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

## API 导出模式

通过 `gomelo.go` 聚合导出，使用类型别名：
```go
type Session = lib.Session
func NewApp(opts ...lib.AppOption) *lib.App { return lib.NewApp(opts...) }
```
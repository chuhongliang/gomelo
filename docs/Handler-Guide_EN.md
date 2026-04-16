# Handler Guide

Handlers are the core components for processing client requests.

## Basic Concept

A Handler is a function that takes `*Context`:

```go
func handlerName(ctx *gomelo.Context)
```

## Register Handler

Register via `app.On(route, handler)`:

```go
app.On("connector.entry", handleEntry)
app.On("chat.send", handleChatSend)
app.On("game.battle", handleBattle)
```

## Context Common Methods

### Get Request Data - Bind

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

### Send Response - Response

```go
ctx.Response(map[string]any{
    "code": 0,
    "msg":  "ok",
    "data": map[string]any{
        "uid": ctx.Session().UID(),
    },
})
```

### Get Session

```go
session := ctx.Session()
session.Set("level", 10)
uid := session.UID()
```

### Get Message

```go
msg := ctx.Message()
log.Printf("Route: %s, Body: %v", msg.Route, msg.Body)
```

## Structured Handler

Organize multiple handlers into a struct:

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

Register:

```go
handler := &ConnectorHandler{app: app}
app.On("connector.entry", handler.Entry)
app.On("connector.heartbeat", handler.Heartbeat)
```

## Error Handling

### Use Predefined Errors

```go
ctx.ResponseError(gomelo.ErrUnauthorized)
ctx.ResponseError(gomelo.ErrInvalidRoute)
ctx.ResponseError(gomelo.ErrTimeout)
```

### Custom Error

```go
ctx.ResponseError(fmt.Errorf("custom error: %w", someErr))
```

## Pre-processing - Middleware

Use `app.Before()` to add pre-processing:

```go
app.Before(handleAuth)
app.Before(handleLog)
```

Middleware signature:

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

## Post-processing - AfterFilter

```go
app.After(handleAfter)
```

```go
func handleAfter(ctx *gomelo.Context) {
    log.Printf("Request completed: %s", ctx.Route)
}
```

## Complete Example

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

## Route Convention

Recommended format: `serverType.handler`

| Route | Description |
|-------|-------------|
| `connector.entry` | Connector entry |
| `connector.heartbeat` | Heartbeat check |
| `chat.send` | Chat send |
| `chat.join` | Join room |
| `game.start` | Game start |
| `game.move` | Game move |

## Auto-Registration (Recommended)

Use codegen to automatically scan and register Handlers - no manual `app.On()` calls needed.

### Directory Structure

```
game-server/app/servers/
  {serverType}/
    handler/      # Handler directory
      entry.go   # Auto-scanned
    remote/       # RPC directory
    filter/       # Filter directory
    cron/         # Cron directory
```

### Handler Naming Convention

- Type name must end with `Handler` or `handler`
- Methods take `*lib.Context` parameter

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

### Run Code Generation

```bash
go run ./cmd/codegen ./game-server/app/servers
```

Generates `servers_gen.go` with all Handlers and Remotes auto-registered.

### Generated File Example

```go
// servers_gen.go (auto-generated)
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

### Route Rules

| Handler Method | Auto-generated Route |
|----------------|----------------------|
| `EntryHandler.Entry` | `connector.entryHandler.entry` |
| `ChatHandler.Send` | `connector.chatHandler.send` |

Combine with `app.Route()` to customize route prefix.

### Init Callback

If Handler implements `Init(app *lib.App)`, it will be called automatically after registration:

```go
func (h *EntryHandler) Init(app *lib.App) {
    h.app = app
}
```
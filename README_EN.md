# Gomelo

A high-performance distributed game server framework written in Go, inspired by Node.js Pomelo architecture.

## Features

- **Distributed Architecture** - Multi-node deployment with frontend/backend separation
- **High-Performance RPC** - Connection pool reuse, async message forwarding with bidirectional tracing
- **Type Safe** - Strongly typed Filter interfaces and Handler signatures
- **Service Discovery** - Master coordination + Registry dual mode
- **Load Balancing** - Round-robin, consistent hash, weighted random strategies
- **Batch Broadcast** - Async batch push, supports UID/ID grouping
- **Production Ready** - Circuit breaker, rate limiting, metrics, health checks, tracing
- **Graceful Shutdown** - Timeout control ensuring task completion
- **Hot Config Reload** - File watching with automatic reload

## Requirements

- Go 1.21+

## Quick Start

### 1. Install CLI

```bash
git clone https://github.com/gomelo/gomelo.git
cd gomelo
go build -o bin/gomelo ./cmd/cli
```

### 2. Global Install (Optional)

```bash
# Linux/Mac
sudo cp bin/gomelo /usr/local/bin/

# Windows (PowerShell Admin)
Copy bin\gomelo.exe C:\Windows\System32\
```

### 3. Initialize Project

```bash
gomelo init mygame
cd mygame
go mod tidy
```

### 4. Start Project

```bash
go run .
```

## Project Structure

```
game-project/
├── game-server/           # Game server
│   ├── main.go
│   ├── go.mod
│   ├── config/
│   │   ├── servers.json
│   │   ├── log.json
│   │   └── master.json
│   ├── servers/          # Server definitions
│   │   ├── connector/
│   │   ├── gate/
│   │   ├── chat/
│   │   └── game/
│   ├── components/      # Shared components
│   ├── cmd/admin/        # Admin monitor
│   └── logs/            # Log directory
├── web-server/           # Frontend static files
│   └── public/
│       ├── index.html
│       └── js/client.js
```

## Example Code

### Minimal Entry (main.go)

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

	app.On("connector.entry", handleEntry)

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

### Session Management

```go
func handleEntry(ctx *gomelo.Context) {
	session := ctx.Session()

	// Bind user ID
	session.Bind("user-123")

	// Store data
	session.Set("level", 10)
	session.Set("name", "player")

	// Get data
	uid := session.UID()          // "user-123"
	level := session.Get("level") // 10

	// Kick player
	session.Close()
}
```

### Message Broadcast

```go
func handleChatSend(ctx *gomelo.Context) {
	var req struct {
		Msg    string `json:"msg"`
		RoomID string `json:"roomId"`
	}
	ctx.Bind(&req)

	uid := ctx.Session().Get("uid")

	// Broadcast to specific room
	broadcast := gomelo.NewBroadcast("room." + req.RoomID)
	broadcast.BroadcastTo([]string{"user-1", "user-2"}, "chat.message", map[string]any{
		"uid":  uid,
		"msg":  req.Msg,
		"time": time.Now().Unix(),
	})
}
```

### RPC Call

```go
func handleForwardToChat(ctx *gomelo.Context) {
	var req struct {
		Target string `json:"target"`
		Msg    string `json:"msg"`
	}
	ctx.Bind(&req)

	// Forward to other server
	forward := gomelo.NewForwarder(app, selector)
	forward.Forward(ctx.Session(), ctx.Message(), serverInfo)
}
```

### Hot Config Reload

```go
app := gomelo.NewApp()

// Enable hot config reload
watcher, _ := config.NewWatcher("config.json")
watcher.Watch(func(cfg *config.Config) {
	log.Printf("Config reloaded: %+v", cfg)
	app.Set("config", cfg)
})
```

## Distributed Architecture

```
                        ┌─────────────┐
                        │   Master    │  ← Service Coordination
                        └─────────────┘
                               │
    ┌──────────────────────────┼──────────────────────────┐
    │                          │                          │
┌───▼────┐              ┌──────▼──────┐              ┌──────▼──────┐
│connector│              │  connector  │              │  connector  │  ← Frontend Layer
│(Frontend)│             │  (Frontend) │              │  (Frontend) │
└────┬────┘              └──────┬──────┘              └──────┬──────┘
     │                          │                              │
     └──────────────────────────┼──────────────────────────────┘
                                │ RPC
                    ┌───────────┼───────────┐
                    │           │           │
              ┌─────▼─────┐┌───▼────┐┌─────▼─────┐
              │    chat    ││   game  ││   auth   │  ← Backend Layer
              └───────────┘└─────────┘└─────────┘
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `gomelo init <name>` | Initialize new project |
| `gomelo add <type>` | Add server type (connector/chat/gate/auth/game/match) |
| `gomelo start` | Start application |
| `gomelo build` | Build application |
| `gomelo clean` | Clean build artifacts |
| `gomelo -v` | Show version |
| `gomelo -h` | Show help |

## Auto Route Registration

Use codegen to automatically scan server code and generate registration:

```bash
go run ./cmd/codegen ./servers
```

Scans `servers/{serverType}/handler/` and `servers/{serverType}/remote/` directories, auto-registering all Handler and Remote methods.

See [Handler-Guide.md](docs/Handler-Guide_EN.md) for details.

## Core API

### App

| Method | Description |
|--------|-------------|
| `NewApp(opts...)` | Create app instance |
| `WithHost(host)` | Set listen address |
| `WithPort(port)` | Set listen port |
| `WithServerID(id)` | Set server ID |
| `WithMasterAddr(addr)` | Set Master address |
| `Configure(env, serverType)` | Configure server type |
| `On(route, handler)` | Register route handler |
| `Before(filter)` | Register pre-filter |
| `After(filter)` | Register post-filter |
| `Start(cb)` | Start app |
| `Stop()` | Stop app |
| `Wait()` | Block waiting for signals |

### Context

| Method | Description |
|--------|-------------|
| `Session()` | Get current Session |
| `Message()` | Get current Message |
| `Bind(v)` | Parse request data |
| `Response(v)` | Send response |
| `ResponseError(err)` | Send error response |
| `Next()` | Call next handler |

### Session

| Method | Description |
|--------|-------------|
| `ID()` | Get session ID |
| `UID()` | Get bound user ID |
| `Bind(uid)` | Bind user ID |
| `Set(key, val)` | Store data |
| `Get(key)` | Get data |
| `Remove(key)` | Delete data |
| `Push(key, val, cb)` | Push data to client |
| `Close()` | Close session |
| `OnClose(handler)` | Register close callback |

### Server

| Method | Description |
|--------|-------------|
| `SetFrontend(v)` | Set as frontend server |
| `SetPort(port)` | Set port |
| `SetHost(host)` | Set address |
| `SetServerType(t)` | Set server type |
| `OnConnection(fn)` | Connection callback |
| `OnMessage(fn)` | Message callback |
| `OnClose(fn)` | Close callback |

## Directory Structure

```
gomelo/
├── gomelo.go           # Entry, exports all public APIs
├── lib/                 # Core library
│   ├── app.go          # Application
│   ├── session.go      # Session management
│   ├── context.go      # Request context
│   ├── router.go       # Router
│   ├── pipeline.go    # Middleware pipeline
│   ├── event.go        # Event emitter
│   ├── error.go        # Error definitions
│   ├── lifecycle.go    # Lifecycle interface
│   ├── circuitbreaker.go # Circuit breaker
│   ├── ratelimit.go    # Rate limiting
│   ├── metrics.go      # Metrics
│   ├── health.go       # Health check
│   ├── tracing.go      # Tracing
│   └── shutdown.go     # Graceful shutdown
├── rpc/                 # RPC system
│   ├── client.go       # RPC client + connection pool
│   └── server.go       # RPC server
├── connector/           # Network connector
├── master/             # Master server
├── registry/           # Service registry
├── server_registry/     # Server registry
├── selector/           # Load balancer
├── forward/            # Message forwarder
├── broadcast/           # Broadcast service
├── pool/               # Connection pool + WorkerPool
├── scheduler/          # Cron job scheduler
├── loader/             # Handler/Remote loader
├── config/             # Config system
├── codec/              # Message codec
├── filter/             # Filter interface
├── route/              # Route compression
├── logger/             # Logger
├── plugin/             # Plugin system
├── component/          # Component interface
├── websocket/          # WebSocket support
└── cmd/                # CLI tools
    ├── cli/            # gomelo CLI
    ├── demo/           # Demo
    └── codegen/        # Code generator
```

## Comparison with Node.js Pomelo

| Feature | Node.js Pomelo | gomelo |
|---------|---------------|--------|
| Install | `npm install -g pomelo` | `go build ./cmd/cli` |
| Init | `pomelo init mygame` | `gomelo init mygame` |
| Start | `node start.js` | `go run .` |
| Entry file | `start.js` | `main.go` |
| Handler signature | `function(session, msg, next)` | `func(ctx *Context)` |
| Filter interface | `before/after filter` | `Before/After filter` |
| RPC | `pomelo.rpc.invoke` | `client.Invoke(service, method, args, reply)` |

## Performance

- RPC connection pool reuse: >90%
- Message forwarding latency: <1ms
- Single node connections: 10000+
- Goroutine pooling to avoid unlimited creation

## Documentation

- [中文版](../README.md)
- [Handler Guide](docs/Handler-Guide.md)
- [Getting Started](docs/Getting-Started.md)
- [Session Guide](docs/Session-Guide.md)
- [Distributed Guide](docs/Distributed-Guide.md)
- [API Reference](docs/API.md)

## License

MIT
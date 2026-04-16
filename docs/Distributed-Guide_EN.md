# Distributed Guide

gomelo supports multi-node distributed deployment for large-scale game server architecture.

## Architecture Overview

```
                        ┌─────────────┐
                        │   Master    │  ← Service Coordination
                        │  (port 3040) │
                        └─────────────┘
                               │
    ┌──────────────────────────┼──────────────────────────┐
    │                          │                          │
┌───▼────┐              ┌──────▼──────┐              ┌──────▼──────┐
│connector│              │  connector  │              │  connector  │
│port 3010│              │  port 3011  │              │  port 3012  │  ← Frontend Layer
└────┬────┘              └──────┬──────┘              └──────┬──────┘
     │                           │                          │
     └──────────────────────────┼──────────────────────────┘
                                │ RPC
                    ┌───────────┼───────────┐
                    │           │           │
              ┌─────▼─────┐┌───▼────┐┌─────▼─────┐
              │    chat    ││  game  ││   auth    │  ← Backend Layer
              │  port 3020 ││port3030││ port 3040 │
              └───────────┘└────────┘└───────────┘
```

## Core Components

### Master Server

Master coordinates all servers:

```go
master := master.New(":3040")
master.Start()
```

### Registry

Service registry for service discovery:

```go
reg := registry.New()
```

### Selector

Load balancer:

```go
sel := selector.NewRandomSelector()
// or consistent hash
sel := selector.NewConsistentHashSelector(100, nil)
```

## Configure Multi-Server

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

## Frontend Server (Connector)

Handles client connections:

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

// Register to Master
app.On("connector.entry", handleEntry)

// Forward messages to backend
app.On("chat.send", handleForwardToChat)
```

## Backend Server

Handles business logic:

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

## RPC Call

### Create RPC Client

```go
client := rpc.NewClient(&rpc.ClientOptions{
    Host:    "127.0.0.1",
    Port:    3020,
    MaxConns: 5,
    Timeout:  5 * time.Second,
})
```

### Use Connection Pool

```go
pool := rpc.NewClientPool("127.0.0.1:3020", 10, 1, 5*time.Second)
client, err := pool.GetClient()
if err != nil {
    log.Fatal(err)
}
defer pool.Close()
```

### Call Example

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

### Notify (No Response Call)

```go
err := client.Notify("chat", "Broadcast", map[string]any{
    "roomId": "room-1",
    "msg":    "hello",
})
```

## Message Forwarding

### Forwarder

Message forwarder handles cross-server communication:

```go
forward := forward.NewForwarder(app, selector)

func handleForward(ctx *gomelo.Context) {
    var req struct {
        Target string `json:"target"`
        Route  string `json:"route"`
        Data   any    `json:"data"`
    }
    ctx.Bind(&req)

    // Select target server
    servers := registry.GetServersByType(req.Target)
    if len(servers) == 0 {
        ctx.ResponseError(errors.New("no server available"))
        return
    }

    server := selector.Select(servers)
    forward.Forward(ctx.Session(), ctx.Message(), server)
}
```

## Service Registry and Discovery

### Register Service

```go
// Connect to Master
master := master.New("127.0.0.1:3040")

// Register current server
master.AddServer(&master.ServerInfo{
    ID:         "connector-1",
    ServerType: "connector",
    Host:       "127.0.0.1",
    Port:       3010,
    Frontend:   true,
})
```

### Subscribe Changes

```go
registry.Watch(func(event string, servers []*registry.ServerInfo) {
    log.Printf("Event: %s, Servers: %d", event, len(servers))
    for _, s := range servers {
        log.Printf("  - %s: %s:%d", s.ID, s.Host, s.Port)
    }
})
```

## Load Balancing Strategies

### Random Selection

```go
sel := selector.NewRandomSelector()
server := sel.Select(keys...)
```

### Round Robin

```go
sel := selector.NewRoundRobinSelector()
server := sel.Select(keys...)
```

### Consistent Hash

Suitable for stateful services, reduces data migration:

```go
sel := selector.NewConsistentHashSelector(100, nil) // 100 virtual nodes
sel.AddServer(serverInfo1)
sel.AddServer(serverInfo2)
server := sel.Select(uid) // Same UID routes to same server
```

## Broadcast

### Broadcast All

```go
broadcast := broadcast.NewBroadcast("announcement")
broadcast.Broadcast("server.announcement", map[string]any{
    "msg": "Server maintenance in 5 minutes",
})
```

### Broadcast to Specific Users

```go
broadcast.BroadcastTo([]string{"user-1", "user-2", "user-3"}, "chat.message", map[string]any{
    "from": "admin",
    "msg":  "hello",
})
```

## Complete Examples

### Start Master

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

### Start Connector

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

### Start Chat Server

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

    // Register RPC Handler
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

## Health Check

Heartbeat check automatically removes crashed servers:

```go
master.OnStateChange(func(id string, oldState, newState int) {
    log.Printf("Server %s state changed: %d -> %d", id, oldState, newState)
    if newState == 3 { // Timeout state
        log.Printf("Server %s is down", id)
    }
})
```

## Best Practices

1. **Frontend/Backend Separation** - Connector only does connection management, business logic goes to backend
2. **Use Connection Pool** - Avoid frequently creating RPC connections
3. **Choose Load Balancing Properly** - Use round-robin/random for stateless, consistent hash for stateful
4. **Monitor Server Status** - Subscribe to OnStateChange to respond to server down timely
5. **Graceful Shutdown** - Unregister from Master before shutdown
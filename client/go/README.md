# Gomelo Go Client

A multi-protocol client for Gomelo game server written in Go.

## Protocol Support

| Protocol | Type | Description |
|----------|------|-------------|
| WebSocket | `ws` | WebSocket connection (default) |
| TCP | `tcp` | Direct TCP connection |
| UDP | `udp` | UDP connection (no reconnect) |

## Status

**Complete** - Full feature set with:
- Route compression (Route ID)
- Auto reconnection (TCP/WS)
- Complete error handling
- Connection state management
- Synchronous request support
- Multiple event handlers with target support
- Heartbeat mechanism
- WebSocket handshake
- Gzip compression support (compression.go)

## Installation

```bash
go get github.com/chuhongliang/gomelo/client/go
```

## Usage

### WebSocket (default)

```go
client := gomelo.NewClient(gomelo.ClientOptions{
    Host:     "localhost",
    Port:     3010,
    Protocol: gomelo.ProtocolWebSocket,
})
```

### TCP

```go
client := gomelo.NewClient(gomelo.ClientOptions{
    Host:     "localhost",
    Port:     3010,
    Protocol: gomelo.ProtocolTCP,
})
```

### UDP

```go
client := gomelo.NewClient(gomelo.ClientOptions{
    Host:     "localhost",
    Port:     3010,
    Protocol: gomelo.ProtocolUDP,
})
```

### Full Example

```go
package main

import (
    "fmt"
    "time"
    "github.com/chuhongliang/gomelo/client/go"
)

func main() {
    client := go.NewClient(go.ClientOptions{
        Host:                 "localhost",
        Port:                 3010,
        Protocol:             go.ProtocolTCP,
        Timeout:              5 * time.Second,
        HeartbeatInterval:    30 * time.Second,
        ReconnectInterval:    3 * time.Second,
        MaxReconnectAttempts: 5,
    })

    client.OnConnected(func() {
        fmt.Println("Connected to server")
    })

    client.OnDisconnected(func() {
        fmt.Println("Disconnected from server")
    })

    if err := client.Connect(); err != nil {
        fmt.Printf("Failed to connect: %v\n", err)
        return
    }
    defer client.Disconnect()

    client.On("onChat", func(data interface{}) {
        fmt.Printf("Chat received: %v\n", data)
    }, nil)

    resp, err := client.Request("connector.entry", map[string]interface{}{
        "name": "Player1",
    })
    if err != nil {
        fmt.Printf("Request failed: %v\n", err)
        return
    }
    fmt.Printf("Response: %v\n", resp)

    client.Notify("player.move", map[string]interface{}{
        "x": 100,
        "y": 200,
    })
}
```

## API

### ClientOptions

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| Host | string | required | Server host |
| Port | int | required | Server port |
| Protocol | ProtocolType | `ws` | Connection protocol (`tcp`, `udp`, `ws`) |
| Timeout | time.Duration | 5s | Request timeout |
| HeartbeatInterval | time.Duration | 30s | Heartbeat interval |
| ReconnectInterval | time.Duration | 3s | Reconnect interval |
| MaxReconnectAttempts | int | 5 | Max reconnection attempts (TCP/WS only) |

### ProtocolType Constants

```go
const (
    ProtocolTCP   ProtocolType = "tcp"
    ProtocolUDP   ProtocolType = "udp"
    ProtocolWebSocket ProtocolType = "ws"
)
```

### Methods

| Method | Description |
|--------|-------------|
| `Connect() error` | Connect to server |
| `Disconnect()` | Disconnect from server |
| `Request(route string, msg interface{}) (interface{}, error)` | Send request and wait for response |
| `RequestWithTimeout(route string, msg interface{}, timeout time.Duration) (interface{}, error)` | Send request with custom timeout |
| `Notify(route string, msg interface{}) error` | Send fire-and-forget message |
| `On(event string, callback func(interface{}), target interface{})` | Register event handler |
| `Off(event string, target interface{})` | Unregister event handler |
| `RegisterRoute(route string, routeID uint32)` | Register route ID for compression |
| `GenerateRouteID() uint32` | Generate next route ID |
| `IsConnected() bool` | Check connection status |
| `OnConnected(func())` | Set connected callback |
| `OnDisconnected(func())` | Set disconnected callback |
| `OnError(func(error))` | Set error callback |

## Protocol

This client implements Gomelo's binary protocol:

- **4 bytes length header** (big-endian)
- **1 byte message type** (Request=1, Response=2, Notify=3, Error=4)
- **Route**: String route or 2-byte route ID (for compressed routes)
- **8 bytes sequence number** (big-endian)
- **Body**: JSON encoded message

## License

MIT
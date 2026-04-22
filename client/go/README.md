# Gomelo Go Client

A simple WebSocket client for Gomelo game server written in Go.

## Status

**Complete** - Full feature set with:
- Route compression (Route ID)
- Auto reconnection
- Complete error handling
- Connection state management
- Synchronous request support
- Multiple event handlers with target support

## Installation

Copy `client.go` to your Go project.

## Usage

```go
package main

import (
    "fmt"
    "time"
    "github.com/chuhongliang/gomelo/client/go"
)

func main() {
    client := go.NewClient(go.ClientOptions{
        Host:            "localhost",
        Port:            3010,
        Timeout:         5 * time.Second,
        HeartbeatInterval: 30 * time.Second,
    })

    if err := client.Connect(); err != nil {
        fmt.Printf("Failed to connect: %v\n", err)
        return
    }
    defer client.Disconnect()

    // Register event handlers
    client.On("onChat", func(data interface{}) {
        fmt.Printf("Chat received: %v\n", data)
    }, nil)

    // Make a request
    resp, err := client.Request("connector.entry", map[string]interface{}{
        "name": "Player1",
    })
    if err != nil {
        fmt.Printf("Request failed: %v\n", err)
        return
    }
    fmt.Printf("Response: %v\n", resp)

    // Send notification
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
| Timeout | time.Duration | 5s | Request timeout |
| HeartbeatInterval | time.Duration | 30s | Heartbeat interval |
| ReconnectInterval | time.Duration | 3s | Reconnect interval |
| MaxReconnectAttempts | int | 5 | Max reconnection attempts |

### Methods

| Method | Description |
|--------|-------------|
| `Connect() error` | Connect to server |
| `Disconnect()` | Disconnect from server |
| `Request(route string, msg interface{}) (interface{}, error)` | Send request and wait for response |
| `Notify(route string, msg interface{}) error` | Send fire-and-forget message |
| `On(event string, callback func(interface{}), target interface{})` | Register event handler |
| `Off(event string, target interface{})` | Unregister event handler |
| `IsConnected() bool` | Check connection status |

## Protocol

This client implements Gomelo's binary protocol:

- **Message Type**: 1 byte (Request=1, Response=2, Notify=3, Error=4)
- **Route**: String route or route ID (for compressed routes)
- **Body**: JSON encoded message

## License

MIT

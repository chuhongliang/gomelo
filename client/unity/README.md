# Gomelo Unity Client

Multi-protocol client for Gomelo game server.

## Protocol Support

| Protocol | Description |
|----------|-------------|
| WebSocket | `ws://host:port` (default) |
| TCP | Direct TCP connection |
| UDP | Direct UDP connection (no reconnect) |

## Requirements

- Unity 2021.3+
- .NET Standard 2.0 or .NET Framework 4.6+

## Status

**Complete** - Full feature set with:
- Route compression (Route ID)
- Auto reconnection (TCP/WebSocket)
- Complete error handling
- Connection state management
- Synchronous request support
- Multiple event handlers
- Native WebSocket support (System.Net.WebSockets)
- TCP direct connection
- UDP direct connection

## Installation

Copy the `Gomelo` folder to your Unity project's `Assets` folder.

## Usage

### WebSocket (default)

```csharp
client.Protocol = Network.ProtocolType.WebSocket;
client.Connect();
```

### TCP

```csharp
client.Protocol = Network.ProtocolType.TCP;
client.Connect();
```

### UDP

```csharp
client.Protocol = Network.ProtocolType.UDP;
client.Connect();
```

### Full Example

```csharp
using UnityEngine;
using Gomelo;
using Gomelo.Network;

public class GameClient : MonoBehaviour
{
    private GomeloClient client;

    void Start()
    {
        client = gameObject.AddComponent<GomeloClient>();
        client.Host = "localhost";
        client.Port = 3010;
        client.Protocol = Network.ProtocolType.TCP;
        client.Timeout = 5000;
        client.HeartbeatInterval = 30000;

        client.OnConnected += () => Debug.Log("Connected");
        client.OnDisconnected += () => Debug.Log("Disconnected");
        client.OnError += (err) => Debug.LogError("Error: " + err);

        client.On("onChat", (data) => Debug.Log("Chat: " + data));

        client.Connect();

        client.Request("connector.entry", new { name = "Player1" },
            (data) => Debug.Log("Entry: " + data),
            (err) => Debug.LogError("Entry failed: " + err));

        client.Notify("player.move", new { x = 100, y = 200 });
    }

    void OnDestroy()
    {
        client?.Disconnect();
    }
}
```

## API Reference

### GomeloClient

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| Host | string | localhost | Server host |
| Port | int | 3010 | Server port |
| Protocol | ProtocolType | WebSocket | Connection protocol (WebSocket/TCP/UDP) |
| Timeout | int | 5000 | Request timeout (ms) |
| HeartbeatInterval | int | 30000 | Heartbeat interval (ms) |

### ProtocolType Enum

```csharp
public enum ProtocolType
{
    WebSocket,
    TCP,
    UDP
}
```

### Events

| Event | Description |
|-------|-------------|
| OnConnected | Fired when connected to server |
| OnDisconnected | Fired when disconnected from server |
| OnError | Fired on connection/communication error |
| OnResponse | Fired on any response received |
| OnNotify | Fired on any notify/error message received |

### Methods

| Method | Description |
|--------|-------------|
| `Connect(host, port, protocol)` | Connect to server |
| `Disconnect()` | Disconnect from server |
| `Request(route, body, onSuccess, onError)` | Send request with callback, returns sequence number |
| `Notify(route, body)` | Send fire-and-forget message |
| `On(route, handler)` | Register event handler |
| `Off(route, handler)` | Unregister event handler |
| `RegisterRoute(route, id)` | Register route for compression |
| `IsConnected` | Check connection status |

### Packet.Encode / Packet.Decode

For custom packet handling:

```csharp
byte[] data = Network.Packet.Encode(
    Network.MessageType.Request,
    "connector.entry",
    12345,
    new { name = "Player1" }
);

var packet = Network.Packet.Decode(data);
```

### RouteManager

```csharp
Network.RouteManager.RegisterRoute("connector.entry", 1);
Network.RouteManager.RegisterRoute("player.move", 2);
int routeId = Network.RouteManager.GetRouteId("connector.entry");
string route = Network.RouteManager.GetRoute(1);
```

## Protocol

Implements Gomelo's binary protocol:
- **4 bytes length header** (big-endian)
- **1 byte message type** (Request=1, Response=2, Notify=3, Error=4)
- **Route**: String or 2-byte route ID
- **8 bytes sequence number** (big-endian)
- **JSON body**

## License

MIT
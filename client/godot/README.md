# Gomelo Godot Client

Multi-protocol client for Gomelo game server (Godot 4.x).

## Protocol Support

| Protocol | Description |
|----------|-------------|
| WebSocket | `ws://host:port` (default) |
| TCP | Direct TCP connection |
| UDP | Direct UDP connection (no reconnect) |

## Requirements

- Godot 4.0+
- Network permission enabled in project.godot

## Status

**Complete** - Full feature set with:
- Route compression (Route ID)
- Auto reconnection (TCP/WebSocket)
- Complete error handling
- Connection state management
- Synchronous request support (with await)
- Multiple event handlers with Callable support
- Native WebSocketPeer support
- TCP direct connection
- UDP direct connection

## Installation

1. Copy `client.gd`, `protocol.gd`, and `packet.gd` to your Godot project
2. Add `network/` folder to your project if using the structured version

## Project Configuration

Add to `project.godot`:

```ini
[network]

websocket/enable = true
```

## Usage

### WebSocket (default)

```gdscript
client.protocol = ProtocolType.WEBSOCKET
client.connect_to_server()
```

### TCP

```gdscript
client.protocol = ProtocolType.TCP
client.connect_to_server()
```

### UDP

```gdscript
client.protocol = ProtocolType.UDP
client.connect_to_server()
```

### Full Example

```gdscript
extends Node

var client: GomeloClient

func _ready() -> void:
	client = GomeloClient.new()
	client.host = "localhost"
	client.port = Protocol.DEFAULT_PORT
	client.protocol = ProtocolType.TCP
	client.timeout = Protocol.DEFAULT_TIMEOUT
	client.heartbeat_interval = 30000
	client.reconnect_interval = 3000
	client.max_reconnect_attempts = 5

	client.connected.connect(_on_connected)
	client.disconnected.connect(_on_disconnected)
	client.error.connect(_on_error)
	client.notify.connect(_on_notify)
	client.response.connect(_on_response)

	client.on("onChat", _handle_chat)

	client.connect_to_server()

	var result = await client.request_sync("connector.entry", {"name": "Player1"})
	print("Entry result: ", result)

	client.notify("player.move", {"x": 100, "y": 200})

func _on_connected() -> void:
	print("Connected to server")

func _on_disconnected() -> void:
	print("Disconnected from server")

func _on_error(msg: String) -> void:
	push_error("Error: " + msg)

func _on_notify(route: String, body: Variant) -> void:
	print("Notify %s: %s" % [route, body])

func _on_response(seq: int, body: Variant) -> void:
	print("Response %d: %s" % [seq, body])

func _handle_chat(data: Variant) -> void:
	print("Chat: ", data)
```

## API Reference

### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| host | String | localhost | Server host |
| port | int | 3010 | Server port |
| protocol | ProtocolType | WEBSOCKET | Connection protocol (WEBSOCKET/TCP/UDP) |
| timeout | int | 5000 | Request timeout (ms) |
| heartbeat_interval | int | 30000 | Heartbeat interval (ms) |
| reconnect_interval | int | 3000 | Reconnect interval (ms) |
| max_reconnect_attempts | int | 5 | Max reconnection attempts (not for UDP) |

### ProtocolType Enum

```gdscript
enum ProtocolType {
	WEBSOCKET,
	TCP,
	UDP
}
```

### Signals

| Signal | Description |
|--------|-------------|
| connected() | Fired when connected to server |
| disconnected() | Fired when disconnected from server |
| error(message: String) | Fired on error |
| response(seq: int, body: Variant) | Fired on response received |
| notify(route: String, body: Variant) | Fired on notify/error message received |

### Methods

| Method | Description |
|--------|-------------|
| `connect_to_server(host, port, protocol)` | Connect to server |
| `disconnect_from_server()` | Disconnect from server |
| `request(route, body)` | Send request, returns seq |
| `request_with_callback(route, body, on_success, on_error)` | Send request with callbacks |
| `request_sync(route, body)` | Send request and wait for response (uses await) |
| `notify(route, body)` | Send fire-and-forget message |
| `on(route, handler: Callable)` | Register event handler |
| `off(route, handler: Callable)` | Unregister event handler |
| `off_all(route)` | Unregister all handlers for route |
| `emit_event(route, body)` | Manually emit event |
| `is_connected()` | Check connection status |
| `register_route(route, id)` | Register route for compression |
| `generate_route_id()` | Generate next route ID |

### Event Callback API

```gdscript
# Register callbacks
client.on_connected(func(): print("connected"))
client.on_disconnected(func(): print("disconnected"))
client.on_error(func(msg): print("error: ", msg))

# Using signal handlers
client.connected.connect(_on_connected)
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
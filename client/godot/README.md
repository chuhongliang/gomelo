# Gomelo Godot Client

Godot 4.x client for Gomelo game server with WebSocket support.

## Requirements

- Godot 4.0+
- Network permission enabled in project.godot

## Status

**Complete** - Full feature set with:
- Route compression (Route ID)
- Auto reconnection
- Complete error handling
- Connection state management
- Synchronous request support (with await)
- Multiple event handlers with Callable support
- Native WebSocketPeer support

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

```gdscript
extends Node

var client: GomeloClient

func _ready() -> void:
    client = GomeloClient.new()
    client.host = "localhost"
    client.port = Protocol.DEFAULT_PORT
    client.timeout = Protocol.DEFAULT_TIMEOUT
    client.heartbeat_interval = 30000
    client.reconnect_interval = 3000
    client.max_reconnect_attempts = 5

    client.connected.connect(_on_connected)
    client.disconnected.connect(_on_disconnected)
    client.error.connect(_on_error)
    client.notify.connect(_on_notify)
    client.response.connect(_on_response)

    # Register event handlers
    client.on("onChat", _handle_chat)

    # Connect to server
    client.connect_to_server()

    # Make async request with callback
    client.request_with_callback("connector.entry",
        {"name": "Player1"},
        _on_entry_success,
        _on_entry_error)

    # Or use sync request with await
    var result = await client.request_sync("connector.entry", {"name": "Player1"})

    # Send notification
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

func _on_entry_success(data: Variant) -> void:
    print("Entry success: ", data)

func _on_entry_error(err: Variant) -> void:
    push_error("Entry error: ", err)
```

## API Reference

### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| host | String | localhost | Server host |
| port | int | 3010 | Server port |
| timeout | int | 5000 | Request timeout (ms) |
| heartbeat_interval | int | 30000 | Heartbeat interval (ms) |
| reconnect_interval | int | 3000 | Reconnect interval (ms) |
| max_reconnect_attempts | int | 5 | Max reconnection attempts |

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
| `connect_to_server(host, port)` | Connect to server |
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
- **Message Type**: 1 byte (REQUEST=1, RESPONSE=2, NOTIFY=3, ERROR=4)
- **Route Flag**: 1 byte (ROUTE_ID=0x01, ROUTE_STRING=0x00)
- **Route**: Route ID (2 bytes) or null-terminated string
- **Sequence**: 8 bytes (big-endian)
- **Body**: JSON encoded

## License

MIT
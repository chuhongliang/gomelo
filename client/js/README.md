# Gomelo JavaScript Client

Multi-protocol client for Gomelo game server.

## Protocol Support

| Protocol | Browser | Node.js | Description |
|----------|---------|---------|-------------|
| WebSocket | ✅ | ✅ | WebSocket connection (default) |
| TCP | ❌ | ✅ | Direct TCP connection |
| UDP | ❌ | ✅ | UDP connection (no reconnect) |

## Usage

### WebSocket (Browser & Node.js)

```javascript
const client = new GomeloClient({
  host: 'localhost',
  port: 3010,
  protocol: 'ws'  // default
});
await client.connect();
```

### TCP (Node.js only)

```javascript
const client = new GomeloClient({
  host: 'localhost',
  port: 3010,
  protocol: 'tcp'
});
await client.connect();
```

### UDP (Node.js only)

```javascript
const client = new GomeloClient({
  host: 'localhost',
  port: 3011,
  protocol: 'udp'
});
await client.connect();
```

## API

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| host | string | 'localhost' | Server host |
| port | number | 3010 | Server port |
| protocol | string | 'ws' | Connection protocol ('ws', 'tcp', 'udp') |
| timeout | number | 5000 | Request timeout (ms) |

### Methods

| Method | Description |
|--------|-------------|
| `connect()` | Connect to server |
| `disconnect()` | Disconnect from server |
| `request(route, msg)` | Send request and wait for response |
| `notify(route, msg)` | Send fire-and-forget message |
| `on(event, handler)` | Register event handler |
| `off(event, handler)` | Unregister event handler |
| `registerRoute(route, routeId)` | Register route ID for compression |
| `isConnected()` | Check connection status |

## Protocol

Binary protocol with JSON body:

- **1 byte message type** (Request=1, Response=2, Notify=3, Error=4)
- **1 byte route flag** + route (string or 2-byte ID)
- **8 bytes sequence number**
- **JSON body**

## License

MIT
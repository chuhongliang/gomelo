# Gomelo JavaScript Client

JavaScript/WebSocket client for Gomelo game server.

## Requirements

- Modern browser with WebSocket support
- ES6+ (for import/export)
- pako (for compression support, optional)

## Status

**Complete** - Full feature set with:
- Route compression (Route ID)
- Complete error handling
- Connection state management
- Promise-based async request
- Multiple event handlers
- Heartbeat mechanism
- Binary message handling
- Gzip/Zlib compression (requires pako)

## Installation

### Browser (CDN)

```html
<script src="https://cdn.jsdelivr.net/npm/gomelo-js-client/client.js"></script>
```

### NPM

```bash
npm install gomelo-js-client
```

### Manual

Copy `client.js` to your project.

## Usage

### Browser

```html
<!DOCTYPE html>
<html>
<head>
    <title>Gomelo Client Demo</title>
</head>
<body>
    <script src="client.js"></script>
    <script>
        const client = new GomeloClient({ host: 'localhost', port: 3010 });

        client.on('onChat', (data) => {
            console.log('Chat:', data);
        });

        client.connect()
            .then(() => console.log('Connected'))
            .catch(err => console.error('Connection failed:', err));

        // Request with Promise
        client.request('connector.entry', { name: 'Player1' })
            .then(data => console.log('Response:', data))
            .catch(err => console.error('Request failed:', err));

        // Notify (fire-and-forget)
        client.notify('player.move', { x: 100, y: 200 });

        // Route compression
        client.registerRoute('connector.entry', 1);
        client.registerRoute('player.move', 2);
    </script>
</body>
</html>
```

### ES Module

```javascript
import { GomeloClient, MessageType } from './client.js';

const client = new GomeloClient({
    host: 'localhost',
    port: 3010,
    timeout: 5000
});

await client.connect();

// Event handling
client.on('onChat', (data) => console.log('Chat:', data));
client.on('error', (data) => console.error('Error:', data));

// RPC Request
const resp = await client.request('connector.entry', { name: 'Alice' });
console.log('Response:', resp);

// Fire-and-forget Notify
client.notify('player.move', { x: 100, y: 200 });

// Route ID compression
client.registerRoute('connector.entry', 1);
```

## API Reference

### GomeloClient

#### Constructor Options

```javascript
const client = new GomeloClient({
    host: 'localhost',      // Server host
    port: 3010,            // Server port
    timeout: 5000          // Request timeout (ms)
});
```

#### Connection

| Method | Description |
|--------|-------------|
| `connect()` | Connect to server (returns Promise) |
| `disconnect()` | Disconnect from server |
| `isConnected()` | Check connection status |
| `on(event, handler)` | Register event handler |
| `off(event, handler)` | Unregister event handler |

#### Messaging

| Method | Description |
|--------|-------------|
| `request(route, msg)` | Send request, returns Promise |
| `notify(route, msg)` | Send fire-and-forget message |
| `registerRoute(route, id)` | Register route for compression |
| `registerType(route, type)` | Register type for serialization |

#### Events

| Event | Description |
|-------|-------------|
| `onChat` | Chat message received |
| `onPlayerJoin` | Player joined notification |
| `onPlayerLeave` | Player left notification |
| `error` | Error occurred |

## Message Types

```javascript
const MessageType = {
    Request: 1,    // Request with response
    Response: 2,  // Response to request
    Notify: 3,    // Fire-and-forget
    Error: 4      // Error message
};
```

## Protocol

Implements Gomelo's binary protocol:

- **Message Type**: 1 byte (Request=1, Response=2, Notify=3, Error=4)
- **Route Flag**: 1 byte (RouteID=0x01, RouteString=0x00)
- **Route**: Route ID (2 bytes) or null-terminated string
- **Sequence**: 8 bytes (big-endian)
- **Body**: JSON encoded message

## Compression

```javascript
import { CompressionUtil, GomeloClient } from './client.js';

// Enable compression for large messages
const client = new GomeloClient({ host: 'localhost', port: 3010 });

// Messages larger than threshold will be compressed
const compressed = CompressionUtil.compressGzip(data);
```

## License

MIT
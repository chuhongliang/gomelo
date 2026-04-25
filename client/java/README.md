# Gomelo Java Client

Multi-protocol client for Gomelo game server.

## Protocol Support

| Protocol | Description |
|----------|-------------|
| WebSocket | `ws://host:port` (default) |
| TCP | Direct TCP connection |
| UDP | Direct UDP connection (no reconnect) |

## Requirements

- Java 11+
- Maven 3.6+

## Status

**Complete** - Full feature set with:
- Route compression (Route ID)
- Auto reconnection (TCP/WebSocket)
- Complete error handling
- Connection state management
- Synchronous request support
- Multiple event handlers with target support
- Heartbeat mechanism
- Binary message handling
- Protocol Buffer support (ProtobufCodec)
- Gzip/Zlib compression (CompressionUtil)

## Installation

### Maven

```xml
<dependency>
    <groupId>com.gomelo</groupId>
    <artifactId>gomelo-java-client</artifactId>
    <version>1.3.0</version>
</dependency>
```

### Gradle

```groovy
implementation 'com.gomelo:gomelo-java-client:1.3.0'
```

## Usage

### WebSocket (default)

```java
GomeloClient client = new GomeloClient(new GomeloClient.Options() {{
    host = "localhost";
    port = 3010;
    protocol = GomeloClient.Protocol.WS;
}});
client.connect();
```

### TCP

```java
GomeloClient client = new GomeloClient(new GomeloClient.Options() {{
    host = "localhost";
    port = 3010;
    protocol = GomeloClient.Protocol.TCP;
}});
client.connect();
```

### UDP

```java
GomeloClient client = new GomeloClient(new GomeloClient.Options() {{
    host = "localhost";
    port = 3011;
    protocol = GomeloClient.Protocol.UDP;
}});
client.connect();
```

### Full Example

```java
package com.example;

import com.gomelo.GomeloClient;

public class GameClient {

    private GomeloClient client;

    public void connect() throws Exception {
        client = new GomeloClient(new GomeloClient.Options() {{
            host = "localhost";
            port = 3010;
            protocol = Protocol.TCP;
            timeoutMs = 5000;
            heartbeatIntervalMs = 30000;
        }});

        client.onConnected(v -> System.out.println("Connected"));
        client.onDisconnected(v -> System.out.println("Disconnected"));

        client.on("onChat", data -> {
            System.out.println("Chat received: " + data);
        });

        client.connect();

        Object result = client.requestSync("connector.entry", new Object[]{"Player1"});
        System.out.println("Response: " + result);

        client.notify("player.move", new Object[]{100, 200});
    }

    public static void main(String[] args) throws Exception {
        new GameClient().connect();
    }
}
```

## API Reference

### Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| host | String | localhost | Server host |
| port | int | 3010 | Server port |
| protocol | Protocol | WS | Connection protocol (WS/TCP/UDP) |
| timeoutMs | int | 5000 | Request timeout (ms) |
| heartbeatIntervalMs | int | 30000 | Heartbeat interval (ms) |
| reconnectIntervalMs | int | 3000 | Reconnect interval (ms) |
| maxReconnectAttempts | int | 5 | Max reconnection attempts (TCP/WS) |

### Protocol Enum

```java
public enum Protocol {
    WS("ws"), TCP("tcp"), UDP("udp");
}
```

### Methods

| Method | Description |
|--------|-------------|
| `connect(host, port)` | Connect to server |
| `disconnect()` | Disconnect from server |
| `request(route, msg, callback)` | Send request with callback |
| `requestSync(route, msg)` | Send request synchronously |
| `notify(route, msg)` | Send fire-and-forget message |
| `on(event, handler)` | Register event handler |
| `off(event, target)` | Unregister event handler |
| `isConnected()` | Check connection status |
| `onConnected(Consumer)` | Set connected callback |
| `onDisconnected(Consumer)` | Set disconnected callback |
| `onError(Consumer)` | Set error callback |

### RequestCallback

```java
public interface RequestCallback {
    void onSuccess(Object data);
    void onFailure(Exception error);
}
```

### EventHandler

```java
public interface EventHandler {
    void handle(Object data);
}
```

## Android Support

This client works on Android API 24+ (Android 7.0+).

## Protocol

Implements Gomelo's binary protocol:
- **4 bytes length header** (big-endian)
- **1 byte message type** (Request=1, Response=2, Notify=3, Error=4)
- **Route**: String or 2-byte route ID
- **8 bytes sequence number**
- **JSON body**

## License

MIT
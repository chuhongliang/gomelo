# Gomelo Java Client

Java client for Gomelo game server with WebSocket support.

## Requirements

- Java 11+
- Maven 3.6+

## Installation

### Maven

```xml
<dependency>
    <groupId>com.gomelo</groupId>
    <artifactId>gomelo-java-client</artifactId>
    <version>1.2.0</version>
</dependency>
```

### Gradle

```groovy
implementation 'com.gomelo:gomelo-java-client:1.2.0'
```

## Usage

```java
package com.example;

import com.gomelo.GomeloClient;

public class GameClient {

    private GomeloClient client;

    public void connect() throws Exception {
        client = new GomeloClient();

        client.on("onChat", data -> {
            System.out.println("Chat received: " + data);
        });

        client.connect("localhost", 3010);

        client.request("connector.entry", new Object[]{"Player1"},
            new GomeloClient.RequestCallback() {
                @Override
                public void onSuccess(Object data) {
                    System.out.println("Entry response: " + data);
                }

                @Override
                public void onFailure(Exception error) {
                    System.err.println("Entry failed: " + error);
                }
            });

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
| timeoutMs | int | 5000 | Request timeout (ms) |
| heartbeatIntervalMs | int | 30000 | Heartbeat interval (ms) |
| reconnectIntervalMs | int | 3000 | Reconnect interval (ms) |
| maxReconnectAttempts | int | 5 | Max reconnection attempts |

### Methods

| Method | Description |
|--------|-------------|
| `connect(host, port)` | Connect to server |
| `disconnect()` | Disconnect from server |
| `request(route, msg, callback)` | Send request with callback |
| `notify(route, msg)` | Send fire-and-forget message |
| `on(event, handler)` | Register event handler |
| `off(event, target)` | Unregister event handler |
| `isConnected()` | Check connection status |

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
- **Message Type**: 1 byte (Request=1, Response=2, Notify=3, Error=4)
- **Route**: String or route ID
- **Body**: JSON encoded

## License

MIT

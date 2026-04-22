# Gomelo Cocos Creator Client

TypeScript client for Cocos Creator 3.x game engine.

## Requirements

- Cocos Creator 3.x
- WebSocket support (built into Cocos Creator)
- pako (for compression support, optional)

## Status

**Complete** - Full feature set with:
- Route compression (Route ID)
- Auto reconnection
- Complete error handling
- Connection state management
- Promise-based async request
- Multiple event handlers with target support
- Heartbeat mechanism
- Binary message handling
- Gzip/Zlib compression (requires pako)

## Installation

1. Copy `GomeloClient.ts` to your Cocos Creator project's `assets/scripts` directory
2. The client uses `@ccclass` decorator, so it works seamlessly with Cocos Creator

## Usage

### 1. Add GomeloClient Component

```typescript
import { _decorator, Component, Node } from 'cc';
import { GomeloClient } from './GomeloClient';

const { ccclass, property } = _decorator;

@ccclass('GameManager')
export class GameManager extends Component {

    private client!: GomeloClient;

    start() {
        this.client = this.addComponent(GomeloClient);

        this.client.onConnected = this.onConnected.bind(this);
        this.client.onDisconnected = this.onDisconnected.bind(this);
        this.client.onError = this.onError.bind(this);

        this.client.connect('localhost', 3010);
    }

    onConnected() {
        console.log('Connected to server');

        this.client.request('connector.entry', { name: 'Player1' })
            .then((data) => {
                console.log('Entry response:', data);
            })
            .catch((err) => {
                console.error('Entry failed:', err);
            });
    }

    onDisconnected() {
        console.log('Disconnected from server');
    }

    onError(error: string) {
        console.error('Client error:', error);
    }

    sendChat(message: string) {
        this.client.notify('chat.send', { msg: message });
    }
}
```

### 2. Event System

```typescript
onLoad() {
    this.client.on('onChat', this.handleChat, this);
    this.client.on('onPlayerMove', this.handlePlayerMove, this);
}

onDestroy() {
    this.client.off('onChat', this.handleChat, this);
    this.client.off('onPlayerMove', this.handlePlayerMove, this);
}

handleChat(data: any) {
    console.log('Chat received:', data.msg);
}

handlePlayerMove(data: any) {
    console.log('Player moved to:', data.position);
}
```

### 3. RPC Request-Response

```typescript
async fetchPlayerInfo(playerId: string) {
    try {
        const response = await this.client.request('player.info', {
            playerId: playerId
        });
        return response.data;
    } catch (err) {
        console.error('Failed to fetch player info:', err);
        return null;
    }
}
```

### 4. Notify (Fire-and-forget)

```typescript
sendMoveCommand(position: { x: number, y: number, z: number }) {
    this.client.notify('player.move', {
        position: position,
        timestamp: Date.now()
    });
}
```

## API Reference

### Connection

| Method | Description |
|--------|-------------|
| `connect(host?: string, port?: number)` | Connect to Gomelo server |
| `disconnect()` | Disconnect from server |

### Request/Response

| Method | Description |
|--------|-------------|
| `request(route: string, msg: any): Promise<any>` | Send request and wait for response |
| `notify(route: string, msg: any): void` | Send notification without response |

### Events

| Method | Description |
|--------|-------------|
| `on(event: string, callback: Function, target?: any)` | Register event handler |
| `off(event: string, callback?: Function, target?: any)` | Unregister event handler |
| `emit(event: string, data?: any)` | Emit event to local handlers |

### Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `host` | string | 'localhost' | Server host |
| `port` | number | 3010 | Server port |
| `timeout` | number | 5000 | Request timeout (ms) |
| `heartbeatInterval` | number | 30000 | Heartbeat interval (ms) |
| `reconnectInterval` | number | 3000 | Reconnect interval (ms) |
| `maxReconnectAttempts` | number | 5 | Max reconnection attempts |

### Callbacks

| Callback | Description |
|----------|-------------|
| `onConnected?()` | Called when connected to server |
| `onDisconnected?()` | Called when disconnected from server |
| `onError?(error: string)` | Called on error |

## Protocol

The client uses Gomelo's binary protocol:

- **Message Type**: 1 byte (Request=1, Response=2, Notify=3, Error=4)
- **Route**: String route or route ID
- **Body**: JSON encoded message

## Example Game Scene

```typescript
import { _decorator, Component, Label, Node } from 'cc';
import { GomeloClient } from './GomeloClient';

const { ccclass, property } = _decorator;

@ccclass('ChatRoom')
export class ChatRoom extends Component {

    @property(Label)
    messageLabel!: Label;

    private client!: GomeloClient;

    start() {
        this.client = this.addComponent(GomeloClient);
        this.client.connect('localhost', 3010);

        this.client.onConnected = () => {
            this.messageLabel.string = 'Connected!';
            this.client!.request('room.join', { roomId: 'lobby' });
        };

        this.client.on('onMessage', (data: any) => {
            this.messageLabel.string += `\n${data.from}: ${data.msg}`;
        });
    }

    sendMessage() {
        this.client.notify('room.chat', {
            msg: 'Hello from Cocos!',
            from: 'CocosPlayer'
        });
    }
}
```

## License

MIT

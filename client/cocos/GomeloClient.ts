/**
 * GomeloClient.ts
 * TypeScript client for Cocos Creator 3.x
 * Compatible with Gomelo game server
 */

export enum MessageType {
    Request = 1,
    Response = 2,
    Notify = 3,
    Error = 4
}

export interface ClientOptions {
    host?: string;
    port?: number;
    timeout?: number;
    heartbeatInterval?: number;
    reconnectInterval?: number;
    maxReconnectAttempts?: number;
}

export interface RequestCallback {
    resolve: (value: any) => void;
    reject: (reason?: any) => void;
    timer: number;
    route: string;
}

export interface EventHandler {
    callback: (data: any) => void;
    target?: any;
}

const DEFAULT_PORT = 3010;
const DEFAULT_TIMEOUT = 5000;
const DEFAULT_HEARTBEAT_INTERVAL = 30000;
const DEFAULT_RECONNECT_INTERVAL = 3000;
const DEFAULT_MAX_RECONNECT_ATTEMPTS = 5;

@ccclass('GomeloClient')
export class GomeloClient extends cc.Component {

    public host: string = 'localhost';
    public port: number = DEFAULT_PORT;
    public timeout: number = DEFAULT_TIMEOUT;
    public heartbeatInterval: number = DEFAULT_HEARTBEAT_INTERVAL;
    public reconnectInterval: number = DEFAULT_RECONNECT_INTERVAL;
    public maxReconnectAttempts: number = DEFAULT_MAX_RECONNECT_ATTEMPTS;

    public onConnected?: () => void;
    public onDisconnected?: () => void;
    public onError?: (error: string) => void;

    private _webSocket: WebSocket | null = null;
    private _connected: boolean = false;
    private _seq: number = 0;
    private _requestCallbacks: Map<number, RequestCallback> = new Map();
    private _eventHandlers: Map<string, EventHandler[]> = new Map();
    private _routeToId: Map<string, number> = new Map();
    private _idToRoute: Map<number, string> = new Map();
    private _nextRouteId: number = 0;
    private _heartbeatTimer: number = 0;
    private _reconnectAttempts: number = 0;
    private _reconnectTimer: number = 0;
    private _isReconnecting: boolean = false;

    start() {
        this._setupHeartbeat();
    }

    onDestroy() {
        this.disconnect();
    }

    connect(host?: string, port?: number) {
        if (host) this.host = host;
        if (port && port > 0) this.port = port;

        this._doConnect();
    }

    private _doConnect() {
        if (this._webSocket) {
            this._webSocket.close();
            this._webSocket = null;
        }

        const url = `ws://${this.host}:${this.port}`;
        this._webSocket = new WebSocket(url);
        this._webSocket.binaryType = 'arraybuffer';

        this._webSocket.onopen = this._onOpen.bind(this);
        this._webSocket.onmessage = this._onMessage.bind(this);
        this._webSocket.onerror = this._onError.bind(this);
        this._webSocket.onclose = this._onClose.bind(this);
    }

    private _onOpen() {
        this._connected = true;
        this._reconnectAttempts = 0;
        this._isReconnecting = false;
        this._stopReconnectTimer();

        if (this.onConnected) {
            this.onConnected();
        }
    }

    private _onMessage(event: MessageEvent) {
        if (event.data instanceof ArrayBuffer) {
            const view = new DataView(event.data);
            this._handlePacket(view);
        } else if (typeof event.data === 'string') {
            const bytes = new TextEncoder().encode(event.data);
            const view = new DataView(bytes.buffer);
            this._handlePacket(view);
        }
    }

    private _onError(event: Event) {
        if (this.onError) {
            this.onError('WebSocket error occurred');
        }
    }

    private _onClose(event: CloseEvent) {
        this._connected = false;
        this._stopHeartbeat();

        if (this.onDisconnected) {
            this.onDisconnected();
        }

        this._tryReconnect();
    }

    disconnect() {
        this._isReconnecting = true;
        this._stopHeartbeat();
        this._stopReconnectTimer();

        if (this._webSocket) {
            this._webSocket.close();
            this._webSocket = null;
        }

        this._connected = false;
        this._requestCallbacks.forEach((cb) => {
            clearTimeout(cb.timer);
        });
        this._requestCallbacks.clear();
    }

    registerRoute(route: string, routeId: number) {
        this._routeToId.set(route, routeId);
        this._idToRoute.set(routeId, route);
    }

    request(route: string, msg: any = {}): Promise<any> {
        return new Promise((resolve, reject) => {
            if (!this._connected) {
                reject(new Error('Not connected'));
                return;
            }

            const seq = ++this._seq;
            const data = this._encode(MessageType.Request, route, seq, msg);

            const timer = setTimeout(() => {
                this._requestCallbacks.delete(seq);
                reject(new Error('Request timeout'));
            }, this.timeout);

            this._requestCallbacks.set(seq, { resolve, reject, timer, route });
            this._send(data);
        });
    }

    notify(route: string, msg: any = {}) {
        if (!this._connected) {
            return;
        }

        const data = this._encode(MessageType.Notify, route, 0, msg);
        this._send(data);
    }

    on(event: string, callback: (data: any) => void, target?: any) {
        if (!this._eventHandlers.has(event)) {
            this._eventHandlers.set(event, []);
        }

        const handlers = this._eventHandlers.get(event)!;
        handlers.push({ callback, target });
    }

    off(event: string, callback?: (data: any) => void, target?: any) {
        if (!callback) {
            this._eventHandlers.delete(event);
            return;
        }

        const handlers = this._eventHandlers.get(event);
        if (!handlers) return;

        const index = handlers.findIndex(
            (h) => h.callback === callback && h.target === target
        );

        if (index !== -1) {
            handlers.splice(index, 1);
        }

        if (handlers.length === 0) {
            this._eventHandlers.delete(event);
        }
    }

    emit(event: string, data?: any) {
        const handlers = this._eventHandlers.get(event);
        if (!handlers) return;

        handlers.forEach((handler) => {
            handler.callback.call(handler.target, data);
        });
    }

    private _send(data: ArrayBuffer) {
        if (this._webSocket && this._connected) {
            this._webSocket.send(data);
        }
    }

    private _encode(type: MessageType, route: string, seq: number, msg: any): ArrayBuffer {
        const routeId = this._routeToId.get(route);
        const body = JSON.stringify(msg);
        const bodyBytes = new TextEncoder().encode(body);

        const hasRouteId = routeId !== undefined && type === MessageType.Request;

        let headerSize = 4 + 1 + (hasRouteId ? 2 : (1 + route.length));
        if (type === MessageType.Response) {
            headerSize = 4 + 1 + 4;
        }

        const buffer = new ArrayBuffer(headerSize + bodyBytes.length);
        const view = new DataView(buffer);
        const uint8 = new Uint8Array(buffer);

        let offset = 0;

        view.setUint32(offset, headerSize + bodyBytes.length - 4);
        offset += 4;

        view.setUint8(offset, type);
        offset += 1;

        if (type === MessageType.Response) {
            view.setUint32(offset, seq);
        } else if (hasRouteId) {
            view.setUint16(offset, routeId!);
        } else {
            view.setUint8(offset, route.length);
            offset += 1;
            for (let i = 0; i < route.length; i++) {
                uint8[offset + i] = route.charCodeAt(i);
            }
        }

        uint8.set(bodyBytes, offset);

        return buffer;
    }

    private _decodeHeader(view: DataView): { type: MessageType; offset: number; route: string; seq: number } {
        const type = view.getUint8(0) as MessageType;
        let offset = 1;
        let route = '';
        let seq = 0;

        if (type === MessageType.Response) {
            seq = view.getUint32(offset);
            offset += 4;
        } else {
            const routeLen = view.getUint8(offset);
            offset += 1;

            const routeBytes = new Uint8Array(view.buffer, view.byteOffset + offset, routeLen);
            route = new TextDecoder().decode(routeBytes);
            offset += routeLen;
        }

        return { type, offset, route, seq };
    }

    private _handlePacket(view: DataView) {
        if (view.byteLength < 5) return;

        const length = view.getUint32(0);
        if (length > 64 * 1024 || length === 0) return;

        if (view.byteLength < 4 + length) return;

        const { type, offset, route, seq } = this._decodeHeader(new DataView(view.buffer, view.byteOffset + 4, length));

        const bodyBytes = new Uint8Array(view.buffer, view.byteOffset + 4 + offset, view.byteLength - 4 - offset);
        const body = new TextDecoder().decode(bodyBytes);

        let data: any = null;
        try {
            data = JSON.parse(body);
        } catch (e) {
            data = body;
        }

        switch (type) {
            case MessageType.Response:
                this._handleResponse(seq, data);
                break;
            case MessageType.Request:
                this.emit(route, data);
                break;
            case MessageType.Notify:
                this.emit(route, data);
                break;
            case MessageType.Error:
                this.emit('error', { route, code: data.code || -1, msg: data.msg || 'Unknown error' });
                break;
        }
    }

    private _handleResponse(seq: number, data: any) {
        const callback = this._requestCallbacks.get(seq);
        if (!callback) return;

        clearTimeout(callback.timer);
        this._requestCallbacks.delete(seq);

        callback.resolve(data);
    }

    private _setupHeartbeat() {
        this._stopHeartbeat();
        this._heartbeatTimer = setInterval(() => {
            if (this._connected) {
                this.notify('sys.heartbeat', { ts: Date.now() });
            }
        }, this.heartbeatInterval) as any;
    }

    private _stopHeartbeat() {
        if (this._heartbeatTimer) {
            clearInterval(this._heartbeatTimer);
            this._heartbeatTimer = 0;
        }
    }

    private _tryReconnect() {
        if (this._isReconnecting) return;
        if (this._reconnectAttempts >= this.maxReconnectAttempts) return;

        this._isReconnecting = true;
        this._reconnectAttempts++;

        this._reconnectTimer = setTimeout(() => {
            this._isReconnecting = false;
            this._doConnect();
        }, this.reconnectInterval) as any;
    }

    private _stopReconnectTimer() {
        if (this._reconnectTimer) {
            clearTimeout(this._reconnectTimer);
            this._reconnectTimer = 0;
        }
    }
}

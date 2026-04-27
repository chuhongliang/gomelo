/**
 * Gomelo JavaScript Client
 * Transparent JSON/Protobuf handling - business code receives parsed objects
 */

const MessageType = {
  Request: 1,
  Response: 2,
  Notify: 3,
  Error: 4
};

const RouteFlag = {
  RouteID: 0x01,
  RouteString: 0x00
};

const Protocol = {
  WebSocket: 'ws',
  TCP: 'tcp',
  UDP: 'udp'
};

class GomeloClient {
  constructor(options = {}) {
    this.host = options.host || 'localhost';
    this.port = options.port || 3010;
    this.protocol = options.protocol || Protocol.WebSocket;
    this.timeout = options.timeout || 5000;
    this.socket = null;
    this.connected = false;
    this.requestCallbacks = new Map();
    this.eventHandlers = new Map();
    this.routeToId = new Map();
    this.idToRoute = new Map();
    this.routeIdToCodec = new Map();
    this.routeIdToTypeUrl = new Map();
    this.nextRouteId = 0;
    this.schemaReceived = false;
    this.protobufCodec = null;

    this._tcpSocket = null;
    this._udpSocket = null;
    this._isNode = typeof window === 'undefined';
  }

  async connect() {
    switch (this.protocol) {
      case Protocol.TCP:
        return this._connectTCP();
      case Protocol.UDP:
        return this._connectUDP();
      case Protocol.WebSocket:
      default:
        return this._connectWS();
    }
  }

  async _connectWS() {
    return new Promise((resolve, reject) => {
      this.socket = new WebSocket(`ws://${this.host}:${this.port}`);
      this.socket.binaryType = 'arraybuffer';

      this.socket.onopen = () => {
        this.connected = true;
        resolve();
      };

      this.socket.onerror = (err) => {
        reject(err);
      };

      this.socket.onmessage = (event) => {
        this._handleMessage(new DataView(event.data));
      };

      this.socket.onclose = () => {
        this.connected = false;
      };
    });
  }

  async _connectTCP() {
    if (!this._isNode) {
      throw new Error('TCP protocol is only supported in Node.js');
    }

    const net = require('net');
    return new Promise((resolve, reject) => {
      this._tcpSocket = net.createConnection(this.port, this.host);

      this._tcpSocket.on('connect', () => {
        this.connected = true;
        this._startTCPReading();
        resolve();
      });

      this._tcpSocket.on('error', (err) => {
        this.connected = false;
        reject(err);
      });

      this._tcpSocket.on('close', () => {
        this.connected = false;
      });
    });
  }

  _startTCPReading() {
    let buffer = Buffer.alloc(0);

    this._tcpSocket.on('data', (chunk) => {
      buffer = Buffer.concat([buffer, chunk]);

      while (buffer.length >= 4) {
        const length = buffer.readUInt32BE(0);
        const totalLen = 4 + length;

        if (buffer.length < totalLen) {
          break;
        }

        const data = buffer.slice(4, totalLen);
        buffer = buffer.slice(totalLen);

        this._handleMessage(new DataView(data.buffer, data.byteOffset, data.byteLength));
      }
    });
  }

  async _connectUDP() {
    if (!this._isNode) {
      throw new Error('UDP protocol is only supported in Node.js');
    }

    const dgram = require('dgram');
    return new Promise((resolve, reject) => {
      this._udpSocket = dgram.createSocket('udp4');

      this._udpSocket.on('connect', () => {
        this.connected = true;
        this._startUDPReading();
        resolve();
      });

      this._udpSocket.on('error', (err) => {
        this.connected = false;
        reject(err);
      });

      this._udpSocket.on('close', () => {
        this.connected = false;
      });

      this._udpSocket.connect(this.port, this.host);
    });
  }

  _startUDPReading() {
    this._udpSocket.on('message', (msg, rinfo) => {
      this._handleMessage(new DataView(msg.buffer, msg.byteOffset, msg.byteLength));
    });
  }

  disconnect() {
    this.connected = false;

    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }

    if (this._tcpSocket) {
      this._tcpSocket.end();
      this._tcpSocket = null;
    }

    if (this._udpSocket) {
      this._udpSocket.close();
      this._udpSocket = null;
    }
  }

  registerRoute(route, routeId) {
    this.routeToId.set(route, routeId);
    this.idToRoute.set(routeId, route);
  }

  registerType(route, messageType) {
    this.routeToId.set(route, ++this.nextRouteId);
    this.idToRoute.set(this.nextRouteId, route);
    this[route] = messageType;
  }

  async request(route, msg = {}) {
    return new Promise((resolve, reject) => {
      if (!this.connected) {
        reject(new Error('Not connected'));
        return;
      }

      const seq = Date.now();
      const data = this._encode(MessageType.Request, route, seq, msg);
      const arrayBuffer = this._bufferToArrayBuffer(data);

      const timer = setTimeout(() => {
        this.requestCallbacks.delete(seq);
        reject(new Error('Request timeout'));
      }, this.timeout);

      this.requestCallbacks.set(seq, { resolve, reject, timer, route });
      this._send(arrayBuffer);
    });
  }

  notify(route, msg = {}) {
    if (!this.connected) {
      return Promise.reject(new Error('Not connected'));
    }

    const data = this._encode(MessageType.Notify, route, 0, msg);
    const arrayBuffer = this._bufferToArrayBuffer(data);
    this._send(arrayBuffer);
    return Promise.resolve();
  }

  _send(arrayBuffer) {
    switch (this.protocol) {
      case Protocol.TCP:
      case Protocol.UDP:
        if (this._isNode) {
          const buffer = Buffer.from(arrayBuffer);
          if (this.protocol === Protocol.UDP && this._udpSocket) {
            this._udpSocket.send(buffer);
          } else if (this._tcpSocket) {
            this._tcpSocket.write(buffer);
          }
        }
        break;
      case Protocol.WebSocket:
      default:
        if (this.socket) {
          this.socket.send(arrayBuffer);
        }
        break;
    }
  }

  _bufferToArrayBuffer(buffer) {
    if (Buffer.isBuffer(buffer)) {
      return buffer.buffer.slice(buffer.byteOffset, buffer.byteOffset + buffer.byteLength);
    }
    return buffer;
  }

  _encode(type, route, seq, body) {
    const bodyStr = JSON.stringify(body);
    const bodyBytes = new TextEncoder().encode(bodyStr);
    const routeId = this.routeToId.get(route);

    let headerLen;
    let routePart;

    if (routeId !== undefined) {
      headerLen = 1 + 1 + 2 + 8;
      routePart = new ArrayBuffer(3);
      const dv = new DataView(routePart);
      dv.setUint8(0, RouteFlag.RouteID);
      dv.setUint16(1, routeId, false);
    } else {
      const routeBytes = new TextEncoder().encode(route);
      headerLen = 1 + 1 + routeBytes.length + 1 + 8;
      routePart = new ArrayBuffer(routeBytes.length + 1);
      const dv = new DataView(routePart);
      dv.setUint8(0, RouteFlag.RouteString);
      for (let i = 0; i < routeBytes.length; i++) {
        dv.setUint8(1 + i, routeBytes[i]);
      }
      dv.setUint8(1 + routeBytes.length, 0);
    }

    const buf = new ArrayBuffer(1 + headerLen + bodyBytes.length);
    const dv = new DataView(buf);
    let offset = 0;

    dv.setUint8(offset, type);
    offset += 1;

    const routeView = new DataView(routePart);
    for (let i = 0; i < routePart.byteLength; i++) {
      dv.setUint8(offset, routeView.getUint8(i));
      offset++;
    }

    dv.setBigUint64(offset, BigInt(seq), false);
    offset += 8;

    for (let i = 0; i < bodyBytes.length; i++) {
      dv.setUint8(offset, bodyBytes[i]);
      offset++;
    }

    return buf;
  }

  _decode(dv) {
    const type = dv.getUint8(0);
    let offset = 1;

    const flag = dv.getUint8(offset);
    offset += 1;

    let route;
    let routeId = 0;
    if (flag === RouteFlag.RouteID) {
      routeId = dv.getUint16(offset, false);
      offset += 2;
      route = this.idToRoute.get(routeId);
    } else if (flag === RouteFlag.RouteString) {
      const start = offset;
      while (offset < dv.byteLength && dv.getUint8(offset) !== 0) {
        offset++;
      }
      const routeBytes = new Uint8Array(dv.buffer, dv.byteOffset + start, offset - start);
      route = new TextDecoder().decode(routeBytes);
      offset++;
    }

    if (offset >= dv.byteLength) {
      return { type, route, routeId, seq: 0, body: null };
    }

    const seq = Number(dv.getBigUint64(offset, false));
    offset += 8;

    let bodyBytes = null;
    if (offset < dv.byteLength) {
      bodyBytes = new Uint8Array(dv.buffer, dv.byteOffset + offset, dv.byteLength - offset);
      try {
        const bodyStr = new TextDecoder().decode(bodyBytes);
        const parsed = JSON.parse(bodyStr);
        if (parsed.type === 'schema') {
          this._handleSchema(parsed.data);
          return { type, route, routeId, seq, body: null, isSchema: true };
        }
      } catch {
        // not JSON, will decode later based on codec
      }
    }

    return { type, route, routeId, seq, bodyBytes };
  }

  _handleSchema(data) {
    if (!data || !data.routes) return;
    for (const r of data.routes) {
      this.routeToId.set(r.route, r.id);
      this.idToRoute.set(r.id, r.route);
      if (r.codec) {
        this.routeIdToCodec.set(r.id, r.codec);
        if (r.typeUrl) {
          this.routeIdToTypeUrl.set(r.id, r.typeUrl);
        }
      }
    }
    this.schemaReceived = true;
  }

  _decodeBody(route, routeId, bodyBytes) {
    if (!bodyBytes || bodyBytes.length === 0) return null;
    const codec = this.routeIdToCodec.get(routeId);
    if (codec === 'protobuf' && this.protobufCodec) {
      try {
        return this.protobufCodec.decode(route, bodyBytes);
      } catch (e) {
        // fallback to JSON
      }
    }
    try {
      return JSON.parse(new TextDecoder().decode(bodyBytes));
    } catch {
      return null;
    }
  }

  _handleMessage(dv) {
    const msg = this._decode(dv);

    if (msg.isSchema) {
      return;
    }

    const body = this._decodeBody(msg.route, msg.routeId, msg.bodyBytes);

    switch (msg.type) {
      case MessageType.Response: {
        const cb = this.requestCallbacks.get(msg.seq);
        if (cb) {
          clearTimeout(cb.timer);
          this.requestCallbacks.delete(msg.seq);
          if (body && body.error) {
            cb.reject(new Error(body.error));
          } else {
            cb.resolve(body);
          }
        }
        break;
      }

      case MessageType.Notify:
      case MessageType.Request: {
        const handlers = this.eventHandlers.get(msg.route);
        if (handlers) {
          handlers.forEach(h => h(body));
        }
        break;
      }

      case MessageType.Error: {
        const handlers = this.eventHandlers.get('error');
        if (handlers) {
          handlers.forEach(h => h(body));
        }
        break;
      }
    }
  }

  on(event, handler) {
    if (!this.eventHandlers.has(event)) {
      this.eventHandlers.set(event, []);
    }
    this.eventHandlers.get(event).push(handler);
  }

  off(event, handler) {
    if (!handler) {
      this.eventHandlers.delete(event);
      return;
    }

    const handlers = this.eventHandlers.get(event);
    if (handlers) {
      const index = handlers.indexOf(handler);
      if (index !== -1) {
        handlers.splice(index, 1);
      }
    }
  }

  isConnected() {
    return this.connected;
  }
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { GomeloClient, MessageType, Protocol };
}

if (typeof window !== 'undefined') {
  window.GomeloClient = GomeloClient;
  window.MessageType = MessageType;
  window.Protocol = Protocol;
}

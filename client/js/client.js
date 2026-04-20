/**
 * Gomelo JavaScript Client
 * Binary protocol with JSON body encoding
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

class GomeloClient {
  constructor(options = {}) {
    this.host = options.host || 'localhost';
    this.port = options.port || 3010;
    this.timeout = options.timeout || 5000;
    this.socket = null;
    this.connected = false;
    this.requestCallbacks = new Map();
    this.eventHandlers = new Map();
    this.routeToId = new Map();
    this.idToRoute = new Map();
    this.nextRouteId = 0;
  }

  async connect() {
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
        this.handleMessage(new DataView(event.data));
      };

      this.socket.onclose = () => {
        this.connected = false;
      };
    });
  }

  disconnect() {
    if (this.socket) {
      this.socket.close();
      this.connected = false;
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
      const data = this.encode(MessageType.Request, route, seq, msg);

      const timer = setTimeout(() => {
        this.requestCallbacks.delete(seq);
        reject(new Error('Request timeout'));
      }, this.timeout);

      this.requestCallbacks.set(seq, { resolve, reject, timer, route });
      this.socket.send(data);
    });
  }

  notify(route, msg = {}) {
    if (!this.connected) {
      return Promise.reject(new Error('Not connected'));
    }

    const data = this.encode(MessageType.Notify, route, 0, msg);
    this.socket.send(data);
    return Promise.resolve();
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

  encode(type, route, seq, body) {
    const bodyBytes = new TextEncoder().encode(JSON.stringify(body));
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

  decode(dv) {
    const type = dv.getUint8(0);
    let offset = 1;

    const flag = dv.getUint8(offset);
    offset += 1;

    let route;
    if (flag === RouteFlag.RouteID) {
      const routeId = dv.getUint16(offset, false);
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
      return { type, route, seq: 0, body: null };
    }

    const seq = Number(dv.getBigUint64(offset, false));
    offset += 8;

    let body = null;
    if (offset < dv.byteLength) {
      const bodyBytes = new Uint8Array(dv.buffer, dv.byteOffset + offset, dv.byteLength - offset);
      try {
        body = JSON.parse(new TextDecoder().decode(bodyBytes));
      } catch {
        body = bodyBytes;
      }
    }

    return { type, route, seq, body };
  }

  handleMessage(dv) {
    const msg = this.decode(dv);

    switch (msg.type) {
      case MessageType.Response: {
        const cb = this.requestCallbacks.get(msg.seq);
        if (cb) {
          clearTimeout(cb.timer);
          this.requestCallbacks.delete(msg.seq);
          if (msg.body && msg.body.error) {
            cb.reject(new Error(msg.body.error));
          } else {
            cb.resolve(msg.body);
          }
        }
        break;
      }

      case MessageType.Notify:
      case MessageType.Error: {
        const handlers = this.eventHandlers.get(msg.route);
        if (handlers) {
          handlers.forEach(h => h(msg.body));
        }
        break;
      }
    }
  }

  isConnected() {
    return this.connected;
  }
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { GomeloClient, MessageType };
}

if (typeof window !== 'undefined') {
  window.GomeloClient = GomeloClient;
  window.MessageType = MessageType;
}
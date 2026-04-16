/**
 * Pomelo JavaScript Client
 * @version 1.0.0
 */

class PomeloClient {
  constructor(options = {}) {
    this.host = options.host || 'localhost';
    this.port = options.port || 3010;
    this.timeout = options.timeout || 5000;
    this.context = null;
    this.socket = null;
    this.connected = false;
    this.requestCallbacks = new Map();
    this.notifyCallbacks = new Map();
    this.eventHandlers = new Map();
  }

  async connect() {
    return new Promise((resolve, reject) => {
      this.socket = new WebSocket(`ws://${this.host}:${this.port}`);
      
      this.socket.onopen = () => {
        this.connected = true;
        resolve();
      };
      
      this.socket.onerror = (err) => {
        reject(err);
      };
      
      this.socket.onmessage = (event) => {
        this.handleMessage(JSON.parse(event.data));
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

  async request(route, msg = {}) {
    return new Promise((resolve, reject) => {
      if (!this.connected) {
        reject(new Error('Not connected'));
        return;
      }
      
      const seq = Date.now();
      const req = {
        type: 1,
        route: route,
        seq: seq,
        body: msg
      };
      
      const timer = setTimeout(() => {
        this.requestCallbacks.delete(seq);
        reject(new Error('Request timeout'));
      }, this.timeout);
      
      this.requestCallbacks.set(seq, { resolve, reject, timer });
      this.socket.send(JSON.stringify(req));
    });
  }

  notify(route, msg = {}) {
    if (!this.connected) {
      return Promise.reject(new Error('Not connected'));
    }
    
    const req = {
      type: 2,
      route: route,
      body: msg
    };
    
    this.socket.send(JSON.stringify(req));
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

  handleMessage(msg) {
    switch (msg.type) {
      case 1:
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
        
      case 0:
        const handlers = this.eventHandlers.get(msg.route);
        if (handlers) {
          handlers.forEach(h => h(msg.body));
        }
        break;
    }
  }

  isConnected() {
    return this.connected;
  }
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = PomeloClient;
}

if (typeof window !== 'undefined') {
  window.PomeloClient = PomeloClient;
}
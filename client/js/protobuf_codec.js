/**
 * Gomelo Protobuf Codec for JavaScript Client
 * Handles message framing only - body bytes are passed through as-is
 * Server is responsible for PB serialization/deserialization
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

class ProtobufCodec {
  constructor() {
    this.routeToId = new Map();
    this.idToRoute = new Map();
    this.nextId = 0;
  }

  registerRoute(route, id) {
    this.routeToId.set(route, id);
    this.idToRoute.set(id, route);
  }

  registerMessage(route, id) {
    this.registerRoute(route, id);
  }

  getRouteId(route) {
    return this.routeToId.get(route) || 0;
  }

  getRoute(id) {
    return this.idToRoute.get(id) || '';
  }

  encode(msgType, route, seq, bodyBytes) {
    const routeId = this.getRouteId(route);
    const body = bodyBytes || Buffer.alloc(0);

    let headerLen;
    let routePart;

    if (routeId > 0) {
      headerLen = 1 + 1 + 2 + 8;
      routePart = Buffer.alloc(3);
      routePart[0] = RouteFlag.RouteID;
      routePart.writeUInt16BE(routeId, 1);
    } else {
      const routeBytes = Buffer.from(route, 'utf8');
      headerLen = 1 + 1 + routeBytes.length + 1 + 8;
      routePart = Buffer.alloc(routeBytes.length + 2);
      routePart[0] = RouteFlag.RouteString;
      routeBytes.copy(routePart, 1);
      routePart[routePart.length - 1] = 0;
    }

    const buf = Buffer.alloc(1 + headerLen + body.length);
    let offset = 0;

    buf[offset] = msgType;
    offset += 1;

    routePart.copy(buf, offset);
    offset += routePart.length;

    buf.writeBigUInt64BE(BigInt(seq), offset);
    offset += 8;

    body.copy(buf, offset);

    return buf;
  }

  decode(data) {
    const buf = Buffer.isBuffer(data) ? data : Buffer.from(data);
    if (buf.length < 10) {
      return null;
    }

    const msgType = buf[0];
    let offset = 1;

    const flag = buf[offset];
    offset += 1;

    let route = '';
    let routeId = 0;
    if (flag === RouteFlag.RouteID) {
      routeId = buf.readUInt16BE(offset);
      offset += 2;
      route = this.idToRoute.get(routeId) || '';
    } else if (flag === RouteFlag.RouteString) {
      const start = offset;
      while (offset < buf.length && buf[offset] !== 0) {
        offset++;
      }
      route = buf.toString('utf8', start, offset);
      offset++;
    }

    if (offset + 8 > buf.length) {
      return null;
    }

    const seq = Number(buf.readBigUInt64BE(offset));
    offset += 8;

    const bodyBytes = offset < buf.length ? buf.slice(offset) : Buffer.alloc(0);

    let body = null;
    if (bodyBytes.length > 0) {
      try {
        body = JSON.parse(bodyBytes.toString());
      } catch {
        body = bodyBytes;
      }
    }

    return {
      type: msgType,
      route: route,
      routeId: routeId,
      seq: seq,
      body: body
    };
  }
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { ProtobufCodec, MessageType, RouteFlag };
}

if (typeof window !== 'undefined') {
  window.ProtobufCodec = ProtobufCodec;
  window.MessageType = MessageType;
  window.RouteFlag = RouteFlag;
}

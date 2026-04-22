package com.gomelo;

import com.google.protobuf.Message;
import com.google.protobuf.Parser;

import java.nio.ByteBuffer;
import java.util.HashMap;
import java.util.Map;
import java.util.function.Supplier;

public class ProtobufCodec {

    private final Map<String, Integer> routeToId = new HashMap<>();
    private final Map<Integer, String> idToRoute = new HashMap<>();
    private final Map<String, Parser<? extends Message>> parsers = new HashMap<>();
    private int nextId = 0;

    public void registerRoute(String route, int id) {
        routeToId.put(route, id);
        idToRoute.put(id, route);
    }

    public void registerType(String route, int id, Parser<? extends Message> parser) {
        registerRoute(route, id);
        parsers.put(route, parser);
    }

    public int getRouteId(String route) {
        return routeToId.getOrDefault(route, 0);
    }

    public String getRoute(int id) {
        return idToRoute.getOrDefault(id, "");
    }

    public byte[] encode(int msgType, String route, long seq, Message body) {
        int routeId = routeToId.getOrDefault(route, 0);
        byte[] bodyBytes = body != null ? body.toByteArray() : new byte[0];

        int headerLen;
        if (routeId > 0) {
            headerLen = 1 + 2 + 8;
        } else {
            headerLen = 1 + 1 + route.length() + 1 + 8;
        }

        byte[] result = new byte[4 + headerLen + bodyBytes.length];

        int totalLen = headerLen + bodyBytes.length;
        result[0] = (byte) ((totalLen >> 24) & 0xFF);
        result[1] = (byte) ((totalLen >> 16) & 0xFF);
        result[2] = (byte) ((totalLen >> 8) & 0xFF);
        result[3] = (byte) (totalLen & 0xFF);

        int offset = 4;
        result[offset++] = (byte) msgType;

        if (routeId > 0) {
            result[offset++] = 0x01;
            result[offset++] = (byte) ((routeId >> 8) & 0xFF);
            result[offset++] = (byte) (routeId & 0xFF);
        } else {
            result[offset++] = 0x00;
            for (int i = 0; i < route.length(); i++) {
                result[offset++] = (byte) route.charAt(i);
            }
            result[offset++] = 0;
        }

        result[offset++] = (byte) ((seq >> 56) & 0xFF);
        result[offset++] = (byte) ((seq >> 48) & 0xFF);
        result[offset++] = (byte) ((seq >> 40) & 0xFF);
        result[offset++] = (byte) ((seq >> 32) & 0xFF);
        result[offset++] = (byte) ((seq >> 24) & 0xFF);
        result[offset++] = (byte) ((seq >> 16) & 0xFF);
        result[offset++] = (byte) ((seq >> 8) & 0xFF);
        result[offset++] = (byte) (seq & 0xFF);

        System.arraycopy(bodyBytes, 0, result, offset, bodyBytes.length);

        return result;
    }

    public DecodedMessage decode(byte[] data, int offset, int length) {
        if (length < 5) {
            return null;
        }

        int totalLen = ((data[offset] & 0xFF) << 24) | ((data[offset + 1] & 0xFF) << 16) |
                       ((data[offset + 2] & 0xFF) << 8) | (data[offset + 3] & 0xFF);

        if (totalLen > 64 * 1024 || totalLen == 0 || totalLen + 4 > length) {
            return null;
        }

        int msgType = data[offset + 4];
        int pos = offset + 5;

        String route = null;
        int responseSeq = 0;

        if (msgType == GomeloClient.MessageType.Response.getValue()) {
            if (pos + 4 > offset + length) return null;
            responseSeq = ((data[pos] & 0xFF) << 24) | ((data[pos + 1] & 0xFF) << 16) |
                         ((data[pos + 2] & 0xFF) << 8) | (data[pos + 3] & 0xFF);
            pos += 4;
        } else {
            if (pos >= offset + length) return null;
            if (data[pos] == 0x01) {
                pos++;
                if (pos + 2 > offset + length) return null;
                int routeId = (data[pos] << 8) | data[pos + 1];
                pos += 2;
                route = getRoute(routeId);
            } else if (data[pos] == 0x00) {
                pos++;
                int start = pos;
                while (pos < offset + length && data[pos] != 0) pos++;
                route = new String(data, start, pos - start);
                pos++;
            }
        }

        if (pos > offset + length) return null;

        int bodyLen = offset + length - pos;
        byte[] bodyBytes = new byte[bodyLen];
        System.arraycopy(data, pos, bodyBytes, 0, bodyLen);

        Message bodyMsg = null;
        if (route != null && parsers.containsKey(route) && bodyBytes.length > 0) {
            try {
                @SuppressWarnings("unchecked")
                Parser<Message> parser = (Parser<Message>) parsers.get(route);
                bodyMsg = parser.parseFrom(bodyBytes);
            } catch (Exception e) {
                // fallback to raw bytes
            }
        }

        return new DecodedMessage(msgType, route, responseSeq, bodyMsg, bodyBytes);
    }

    public static class DecodedMessage {
        public final int type;
        public final String route;
        public final long seq;
        public final Message body;
        public final byte[] rawBody;

        public DecodedMessage(int type, String route, long seq, Message body, byte[] rawBody) {
            this.type = type;
            this.route = route;
            this.seq = seq;
            this.body = body;
            this.rawBody = rawBody;
        }
    }
}
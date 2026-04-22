package com.gomelo;

import com.google.gson.Gson;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;

import java.nio.ByteBuffer;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledFuture;
import java.util.concurrent.TimeUnit;

public class GomeloClient {

    public enum MessageType {
        Request(1),
        Response(2),
        Notify(3),
        Error(4);

        private final int value;

        MessageType(int value) {
            this.value = value;
        }

        public int getValue() {
            return value;
        }

        public static MessageType fromValue(int value) {
            for (MessageType type : values()) {
                if (type.value == value) {
                    return type;
                }
            }
            return Request;
        }
    }

    public interface RequestCallback {
        void onSuccess(Object data);
        void onFailure(Exception error);
    }

    public interface EventHandler {
        void handle(Object data);
    }

    public static class Options {
        public String host = "localhost";
        public int port = 3010;
        public int timeoutMs = 5000;
        public int heartbeatIntervalMs = 30000;
        public int reconnectIntervalMs = 3000;
        public int maxReconnectAttempts = 5;
    }

    private Options opts;
    private WebSocketClient ws;
    private boolean connected = false;
    private int seq = 0;
    private final Map<Integer, RequestCallback> pendingCallbacks = new ConcurrentHashMap<>();
    private final Map<String, Map<Object, EventHandler>> eventHandlers = new ConcurrentHashMap<>();
    private final Map<String, Integer> routeToId = new ConcurrentHashMap<>();
    private final Map<Integer, String> idToRoute = new ConcurrentHashMap<>();
    private final Gson gson = new Gson();
    private ScheduledExecutorService scheduler;
    private ScheduledFuture<?> heartbeatTask;

    public GomeloClient() {
        this(new Options());
    }

    public GomeloClient(Options opts) {
        this.opts = opts;
        this.scheduler = Executors.newSingleThreadScheduledExecutor();
    }

    public void connect() throws Exception {
        String url = "ws://" + opts.host + ":" + opts.port;

        ws = new WebSocketClient(url, this);
        ws.connect();

        int attempts = 0;
        while (!connected && attempts < 100) {
            Thread.sleep(50);
            attempts++;
        }

        if (connected) {
            startHeartbeat();
        }
    }

    public void disconnect() {
        stopHeartbeat();

        if (ws != null) {
            ws.close();
            ws = null;
        }

        connected = false;
        pendingCallbacks.clear();
    }

    public void registerRoute(String route, int routeId) {
        routeToId.put(route, routeId);
        idToRoute.put(routeId, route);
    }

    public void request(String route, Object msg, RequestCallback callback) {
        if (!connected) {
            callback.onFailure(new Exception("Not connected"));
            return;
        }

        int currentSeq = ++seq;
        pendingCallbacks.put(currentSeq, callback);

        try {
            byte[] data = encode(MessageType.Request, route, currentSeq, msg);
            ws.send(data);

            scheduler.schedule(() -> {
                if (pendingCallbacks.remove(currentSeq) != null) {
                    callback.onFailure(new Exception("Request timeout"));
                }
            }, opts.timeoutMs, TimeUnit.MILLISECONDS);
        } catch (Exception e) {
            pendingCallbacks.remove(currentSeq);
            callback.onFailure(e);
        }
    }

    public void notify(String route, Object msg) {
        if (!connected) {
            return;
        }

        try {
            byte[] data = encode(MessageType.Notify, route, 0, msg);
            ws.send(data);
        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    public void on(String event, EventHandler handler) {
        on(event, handler, null);
    }

    public void on(String event, EventHandler handler, Object target) {
        eventHandlers.computeIfAbsent(event, k -> new ConcurrentHashMap<>())
                     .put(target, handler);
    }

    public void off(String event, Object target) {
        Map<Object, EventHandler> handlers = eventHandlers.get(event);
        if (handlers != null) {
            handlers.remove(target);
        }
    }

    public void emit(String event, Object data) {
        Map<Object, EventHandler> handlers = eventHandlers.get(event);
        if (handlers != null) {
            handlers.values().forEach(h -> h.handle(data));
        }
    }

    public boolean isConnected() {
        return connected;
    }

    public void setConnected(boolean connected) {
        this.connected = connected;
    }

    private void startHeartbeat() {
        heartbeatTask = scheduler.scheduleAtFixedRate(
            () -> {
                if (connected) {
                    notify("sys.heartbeat", new Object[]{});
                }
            },
            opts.heartbeatIntervalMs,
            opts.heartbeatIntervalMs,
            TimeUnit.MILLISECONDS
        );
    }

    private void stopHeartbeat() {
        if (heartbeatTask != null) {
            heartbeatTask.cancel(false);
            heartbeatTask = null;
        }
    }

    void handleMessage(byte[] data) {
        if (data == null || data.length < 5) {
            return;
        }

        int length = ((data[0] & 0xFF) << 24) | ((data[1] & 0xFF) << 16) |
                     ((data[2] & 0xFF) << 8) | (data[3] & 0xFF);

        if (length > 64 * 1024 || length == 0) {
            return;
        }

        MessageType msgType = MessageType.fromValue(data[4]);
        int offset = 5;

        String route = null;
        int responseSeq = 0;

        if (msgType == MessageType.Response) {
            responseSeq = ((data[offset] & 0xFF) << 24) | ((data[offset + 1] & 0xFF) << 16) |
                          ((data[offset + 2] & 0xFF) << 8) | (data[offset + 3] & 0xFF);
            offset += 4;
        } else {
            int routeLen = data[offset] & 0xFF;
            offset++;
            route = new String(data, offset, routeLen);
            offset += routeLen;
        }

        byte[] body = new byte[data.length - offset];
        System.arraycopy(data, offset, body, 0, body.length);
        Object msgData = gson.fromJson(new String(body), Object.class);

        switch (msgType) {
            case Response:
                RequestCallback cb = pendingCallbacks.remove(responseSeq);
                if (cb != null) {
                    cb.onSuccess(msgData);
                }
                break;
            case Request:
            case Notify:
                emit(route, msgData);
                break;
            case Error:
                emit("error", msgData);
                break;
        }
    }

    private byte[] encode(MessageType msgType, String route, int seq, Object msg) {
        String body = gson.toJson(msg);
        byte[] bodyBytes = body.getBytes();

        boolean hasRouteId = msgType == MessageType.Request && routeToId.containsKey(route);

        int headerLen;
        if (msgType == MessageType.Response) {
            headerLen = 1 + 4;
        } else if (hasRouteId) {
            headerLen = 1 + 2;
        } else {
            headerLen = 1 + 1 + route.length();
        }

        byte[] result = new byte[4 + headerLen + bodyBytes.length];

        int totalLen = headerLen + bodyBytes.length;
        result[0] = (byte) ((totalLen >> 24) & 0xFF);
        result[1] = (byte) ((totalLen >> 16) & 0xFF);
        result[2] = (byte) ((totalLen >> 8) & 0xFF);
        result[3] = (byte) (totalLen & 0xFF);

        int offset = 4;
        result[offset++] = (byte) msgType.getValue();

        if (msgType == MessageType.Response) {
            result[offset++] = (byte) ((seq >> 24) & 0xFF);
            result[offset++] = (byte) ((seq >> 16) & 0xFF);
            result[offset++] = (byte) ((seq >> 8) & 0xFF);
            result[offset++] = (byte) (seq & 0xFF);
        } else if (hasRouteId) {
            int routeId = routeToId.get(route);
            result[offset++] = (byte) ((routeId >> 8) & 0xFF);
            result[offset++] = (byte) (routeId & 0xFF);
        } else {
            result[offset++] = (byte) route.length();
            for (int i = 0; i < route.length(); i++) {
                result[offset++] = (byte) route.charAt(i);
            }
        }

        System.arraycopy(bodyBytes, 0, result, offset, bodyBytes.length);

        return result;
    }
}

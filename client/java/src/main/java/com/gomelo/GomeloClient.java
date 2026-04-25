package com.gomelo;

import java.io.*;
import java.net.*;
import java.nio.ByteBuffer;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.Consumer;

public class GomeloClient {

    public enum Protocol {
        WS("ws"), TCP("tcp"), UDP("udp");

        private final String value;

        Protocol(String value) {
            this.value = value;
        }

        public String getValue() {
            return value;
        }
    }

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
        public Protocol protocol = Protocol.WS;
        public int timeoutMs = 5000;
        public int heartbeatIntervalMs = 30000;
        public int reconnectIntervalMs = 3000;
        public int maxReconnectAttempts = 5;
    }

    private Options opts;
    private volatile WebSocketClient ws;
    private volatile TCPClient tcpClient;
    private volatile UDPClient udpClient;
    private final AtomicBoolean connected = new AtomicBoolean(false);
    private final AtomicBoolean closed = new AtomicBoolean(false);
    private final AtomicInteger seq = new AtomicInteger(0);
    private final AtomicInteger nextRouteId = new AtomicInteger(1);
    private final Map<Integer, RequestCallback> pendingCallbacks = new ConcurrentHashMap<>();
    private final Map<String, Map<Object, EventHandler>> eventHandlers = new ConcurrentHashMap<>();
    private final Map<String, Integer> routeToId = new ConcurrentHashMap<>();
    private final Map<Integer, String> idToRoute = new ConcurrentHashMap<>();
    private final com.google.gson.Gson gson = new com.google.gson.Gson();
    private ScheduledExecutorService scheduler;
    private ScheduledFuture<?> heartbeatTask;
    private ScheduledFuture<?> reconnectTask;

    private Consumer<Void> onConnectedCallback;
    private Consumer<Void> onDisconnectedCallback;
    private Consumer<Exception> onErrorCallback;

    public GomeloClient() {
        this(new Options());
    }

    public GomeloClient(Options opts) {
        this.opts = opts;
        this.scheduler = Executors.newScheduledThreadPool(2);
    }

    public GomeloClient onConnected(Consumer<Void> callback) {
        this.onConnectedCallback = callback;
        return this;
    }

    public GomeloClient onDisconnected(Consumer<Void> callback) {
        this.onDisconnectedCallback = callback;
        return this;
    }

    public GomeloClient onError(Consumer<Exception> callback) {
        this.onErrorCallback = callback;
        return this;
    }

    public void connect() throws Exception {
        connect(opts.host, opts.port);
    }

    public void connect(String host, int port) throws Exception {
        opts.host = host;
        opts.port = port;
        closed.set(false);

        switch (opts.protocol) {
            case TCP:
                connectTCP();
                break;
            case UDP:
                connectUDP();
                break;
            case WS:
            default:
                connectWS();
                break;
        }

        int attempts = 0;
        while (!connected.get() && attempts < 100) {
            Thread.sleep(50);
            attempts++;
        }

        if (connected.get()) {
            startHeartbeat();
            if (opts.protocol != Protocol.UDP) {
                startReconnectLoop();
            }
        }
    }

    private void connectWS() throws Exception {
        String url = "ws://" + opts.host + ":" + opts.port;

        if (ws != null) {
            try {
                ws.closeBlocking();
            } catch (Exception e) {
            }
        }

        ws = new WebSocketClient(url, this);
        ws.connect();
    }

    private void connectTCP() throws Exception {
        if (tcpClient != null) {
            tcpClient.close();
        }
        tcpClient = new TCPClient(opts.host, opts.port, this);
        tcpClient.connect();
    }

    private void connectUDP() throws Exception {
        if (udpClient != null) {
            udpClient.close();
        }
        udpClient = new UDPClient(opts.host, opts.port, this);
        udpClient.connect();
    }

    public void disconnect() {
        closed.set(true);
        stopHeartbeat();
        stopReconnect();

        if (ws != null) {
            try {
                ws.closeBlocking();
            } catch (Exception e) {
            }
            ws = null;
        }

        if (tcpClient != null) {
            tcpClient.close();
            tcpClient = null;
        }

        if (udpClient != null) {
            udpClient.close();
            udpClient = null;
        }

        connected.set(false);
        pendingCallbacks.values().forEach(cb -> cb.onFailure(new Exception("disconnected")));
        pendingCallbacks.clear();
    }

    public int generateRouteId() {
        return nextRouteId.getAndIncrement();
    }

    public void registerRoute(String route, int routeId) {
        routeToId.put(route, routeId);
        idToRoute.put(routeId, route);
    }

    public Object requestSync(String route, Object msg) throws Exception {
        final Object[] result = new Object[1];
        final Exception[] error = new Exception[1];
        final boolean[] done = new boolean[1];

        request(route, msg, new RequestCallback() {
            @Override
            public void onSuccess(Object data) {
                result[0] = data;
                done[0] = true;
            }

            @Override
            public void onFailure(Exception e) {
                error[0] = e;
                done[0] = true;
            }
        });

        long start = System.currentTimeMillis();
        while (!done[0] && System.currentTimeMillis() - start < opts.timeoutMs) {
            Thread.sleep(10);
        }

        if (error[0] != null) {
            throw error[0];
        }
        if (!done[0]) {
            throw new Exception("Request timeout");
        }

        return result[0];
    }

    public void request(String route, Object msg, RequestCallback callback) {
        if (!connected.get()) {
            callback.onFailure(new Exception("Not connected"));
            return;
        }

        int currentSeq = seq.incrementAndGet();
        pendingCallbacks.put(currentSeq, callback);

        try {
            byte[] data = encode(MessageType.Request, route, currentSeq, msg);
            send(data);

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
        if (!connected.get()) {
            return;
        }

        try {
            byte[] data = encode(MessageType.Notify, route, 0, msg);
            send(data);
        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    private void send(byte[] data) throws Exception {
        switch (opts.protocol) {
            case TCP:
                if (tcpClient != null) {
                    tcpClient.send(data);
                }
                break;
            case UDP:
                if (udpClient != null) {
                    udpClient.send(data);
                }
                break;
            case WS:
            default:
                if (ws != null) {
                    ws.send(data);
                }
                break;
        }
    }

    public void on(String event, EventHandler handler) {
        on(event, handler, null);
    }

    public void on(String event, EventHandler handler, Object target) {
        eventHandlers.computeIfAbsent(event, k -> new ConcurrentHashMap<>())
                     .put(target != null ? target : handler, handler);
    }

    public void off(String event) {
        eventHandlers.remove(event);
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
        return connected.get();
    }

    public void setConnected(boolean connected) {
        boolean wasConnected = this.connected.getAndSet(connected);

        if (connected && !wasConnected) {
            if (onConnectedCallback != null) {
                onConnectedCallback.accept(null);
            }
        } else if (!connected && wasConnected) {
            if (onDisconnectedCallback != null) {
                onDisconnectedCallback.accept(null);
            }
        }
    }

    private void startHeartbeat() {
        stopHeartbeat();
        heartbeatTask = scheduler.scheduleAtFixedRate(
            () -> {
                if (connected.get() && !closed.get()) {
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

    private void startReconnectLoop() {
        stopReconnect();
        reconnectTask = scheduler.schedule(() -> {
            if (closed.get() || connected.get()) {
                return;
            }

            for (int attempt = 1; attempt <= opts.maxReconnectAttempts; attempt++) {
                if (closed.get()) {
                    return;
                }

                try {
                    switch (opts.protocol) {
                        case TCP:
                            connectTCP();
                            break;
                        case UDP:
                            connectUDP();
                            break;
                        case WS:
                        default:
                            connectWS();
                            break;
                    }

                    if (connected.get()) {
                        startHeartbeat();
                        return;
                    }
                } catch (Exception e) {
                    if (onErrorCallback != null) {
                        onErrorCallback.accept(e);
                    }
                }

                if (attempt < opts.maxReconnectAttempts) {
                    try {
                        Thread.sleep(opts.reconnectIntervalMs * attempt);
                    } catch (InterruptedException ie) {
                        return;
                    }
                }
            }
        }, opts.reconnectIntervalMs, TimeUnit.MILLISECONDS);
    }

    private void stopReconnect() {
        if (reconnectTask != null) {
            reconnectTask.cancel(false);
            reconnectTask = null;
        }
    }

    public void handleMessage(byte[] data, int offset, int length) {
        if (data == null || length < 5) {
            return;
        }

        int msgLength = ((data[offset] & 0xFF) << 24) | ((data[offset + 1] & 0xFF) << 16) |
                       ((data[offset + 2] & 0xFF) << 8) | (data[offset + 3] & 0xFF);

        if (msgLength > 64 * 1024 || msgLength == 0) {
            return;
        }

        if (msgLength + 4 > length) {
            return;
        }

        MessageType msgType = MessageType.fromValue(data[offset + 4]);
        int pos = offset + 5;

        String route = null;
        int responseSeq = 0;

        if (msgType == MessageType.Response) {
            if (pos + 4 > offset + length) {
                return;
            }
            responseSeq = ((data[pos] & 0xFF) << 24) | ((data[pos + 1] & 0xFF) << 16) |
                          ((data[pos + 2] & 0xFF) << 8) | (data[pos + 3] & 0xFF);
            pos += 4;
        } else {
            if (pos >= offset + length) {
                return;
            }
            int routeLen = data[pos] & 0xFF;
            pos++;
            if (pos + routeLen > offset + length) {
                return;
            }
            route = new String(data, pos, routeLen);
            pos += routeLen;
        }

        if (pos > offset + length) {
            return;
        }

        int bodyLen = offset + length - pos;
        byte[] body = new byte[bodyLen];
        System.arraycopy(data, pos, body, 0, bodyLen);
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

    private byte[] encode(MessageType msgType, String route, int seqVal, Object msg) {
        String body = gson.toJson(msg);
        byte[] bodyBytes = body.getBytes();

        Boolean hasRouteId = msgType == MessageType.Request ? routeToId.containsKey(route) : false;

        int headerLen;
        if (msgType == MessageType.Response) {
            headerLen = 1 + 4;
        } else if (Boolean.TRUE.equals(hasRouteId)) {
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
            result[offset++] = (byte) ((seqVal >> 24) & 0xFF);
            result[offset++] = (byte) ((seqVal >> 16) & 0xFF);
            result[offset++] = (byte) ((seqVal >> 8) & 0xFF);
            result[offset++] = (byte) (seqVal & 0xFF);
        } else if (Boolean.TRUE.equals(hasRouteId)) {
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
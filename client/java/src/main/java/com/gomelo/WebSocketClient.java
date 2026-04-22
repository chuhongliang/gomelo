package com.gomelo;

import org.java_websocket.client.WebSocketClient;
import org.java_websocket.handshake.ServerHandshake;

import java.net.URI;
import java.nio.ByteBuffer;

public class WebSocketClient extends WebSocketClient {

    private final GomeloClient gomeloClient;

    public WebSocketClient(String serverUri, GomeloClient client) {
        super(URI.create(serverUri));
        this.gomeloClient = client;
        setConnectionLostTimeout(0);
        setTcpNoDelay(true);
    }

    @Override
    public void onOpen(ServerHandshake handshakedata) {
        gomeloClient.setConnected(true);
    }

    @Override
    public void onMessage(ByteBuffer buffer) {
        if (buffer.hasArray()) {
            gomeloClient.handleMessage(buffer.array(), buffer.arrayOffset(), buffer.limit());
        } else {
            byte[] data = new byte[buffer.remaining()];
            buffer.get(data);
            gomeloClient.handleMessage(data, 0, data.length);
        }
    }

    @Override
    public void onClose(int code, String reason, boolean remote) {
        gomeloClient.setConnected(false);
    }

    @Override
    public void onError(Exception ex) {
        gomeloClient.onError(ex);
    }

    public void send(byte[] data) {
        try {
            send(data, true);
        } catch (Exception e) {
            e.printStackTrace();
        }
    }
}

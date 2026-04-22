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
    }

    @Override
    public void onOpen(ServerHandshake handshakedata) {
        gomeloClient.setConnected(true);
    }

    @Override
    public void onMessage(String message) {
        gomeloClient.handleMessage(message.getBytes());
    }

    @Override
    public void onMessage(ByteBuffer buffer) {
        byte[] data = buffer.array();
        gomeloClient.handleMessage(data);
    }

    @Override
    public void onClose(int code, String reason, boolean remote) {
        gomeloClient.setConnected(false);
    }

    @Override
    public void onError(Exception ex) {
        ex.printStackTrace();
    }

    public void send(byte[] data) {
        send(data, true);
    }
}

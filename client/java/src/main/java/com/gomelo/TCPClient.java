package com.gomelo;

import java.io.*;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.nio.ByteBuffer;

public class TCPClient {

    private Socket socket;
    private final String host;
    private final int port;
    private final GomeloClient gomeloClient;
    private volatile boolean running = false;
    private Thread readThread;

    public TCPClient(String host, int port, GomeloClient client) {
        this.host = host;
        this.port = port;
        this.gomeloClient = client;
    }

    public void connect() throws IOException {
        socket = new Socket();
        socket.connect(new InetSocketAddress(host, port), 5000);
        socket.setTcpNoDelay(true);
        socket.setKeepAlive(true);
        running = true;

        gomeloClient.setConnected(true);
        startReading();
    }

    private void startReading() {
        readThread = new Thread(() -> {
            byte[] buffer = new byte[65536];
            byte[] messageBuffer = new byte[0];
            int offset = 0;

            try {
                InputStream in = socket.getInputStream();
                while (running && !socket.isClosed()) {
                    int read = in.read(buffer, offset, buffer.length - offset);
                    if (read == -1) {
                        break;
                    }

                    int total = offset + read;
                    messageBuffer = combine(messageBuffer, buffer, 0, total);

                    while (messageBuffer.length >= 4) {
                        int msgLength = ((messageBuffer[0] & 0xFF) << 24) |
                                       ((messageBuffer[1] & 0xFF) << 16) |
                                       ((messageBuffer[2] & 0xFF) << 8) |
                                       (messageBuffer[3] & 0xFF);

                        int totalLen = 4 + msgLength;
                        if (messageBuffer.length < totalLen) {
                            offset = messageBuffer.length;
                            break;
                        }

                        byte[] msgData = new byte[msgLength];
                        System.arraycopy(messageBuffer, 4, msgData, 0, msgLength);
                        gomeloClient.handleMessage(msgData, 0, msgData.length);

                        messageBuffer = new byte[messageBuffer.length - totalLen];
                        offset = 0;
                    }
                }
            } catch (IOException e) {
                if (running) {
                    gomeloClient.setConnected(false);
                }
            }
        });
        readThread.start();
    }

    private byte[] combine(byte[] a, byte[] b, int bOffset, int bLength) {
        byte[] result = new byte[a.length + bLength];
        System.arraycopy(a, 0, result, 0, a.length);
        System.arraycopy(b, bOffset, result, a.length, bLength);
        return result;
    }

    public void send(byte[] data) throws IOException {
        if (socket != null && !socket.isClosed()) {
            ByteBuffer buffer = ByteBuffer.allocate(4 + data.length);
            buffer.putInt(data.length);
            buffer.put(data);
            socket.getOutputStream().write(buffer.array());
        }
    }

    public void close() {
        running = false;
        try {
            if (socket != null) {
                socket.close();
            }
        } catch (IOException e) {
        }
    }
}
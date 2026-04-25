package com.gomelo;

import java.io.IOException;
import java.net.DatagramPacket;
import java.net.DatagramSocket;
import java.nio.ByteBuffer;

public class UDPClient {

    private DatagramSocket socket;
    private final String host;
    private final int port;
    private final GomeloClient gomeloClient;
    private volatile boolean running = false;
    private Thread readThread;

    public UDPClient(String host, int port, GomeloClient client) {
        this.host = host;
        this.port = port;
        this.gomeloClient = client;
    }

    public void connect() throws IOException {
        socket = new DatagramSocket();
        running = true;

        gomeloClient.setConnected(true);
        startReading();
    }

    private void startReading() {
        readThread = new Thread(() {
            byte[] buffer = new byte[65536];
            try {
                while (running && socket != null && !socket.isClosed()) {
                    DatagramPacket packet = new DatagramPacket(buffer, buffer.length);
                    socket.receive(packet);

                    byte[] data = new byte[packet.getLength()];
                    System.arraycopy(packet.getData(), 0, data, 0, packet.getLength());
                    gomeloClient.handleMessage(data, 0, data.length);
                }
            } catch (IOException e) {
                if (running) {
                    gomeloClient.setConnected(false);
                }
            }
        });
        readThread.start();
    }

    public void send(byte[] data) throws IOException {
        if (socket != null && !socket.isClosed()) {
            DatagramPacket packet = new DatagramPacket(data, data.length);
            packet.setAddress(java.net.InetAddress.getByName(host));
            packet.setPort(port);
            socket.send(packet);
        }
    }

    public void close() {
        running = false;
        if (socket != null) {
            socket.close();
        }
    }
}
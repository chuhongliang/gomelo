package com.gomelo.demo;

import com.gomelo.GomeloClient;

public class GameClient {
    private GomeloClient client;

    public void start() throws Exception {
        client = new GomeloClient();
        client.setHost("localhost");
        client.setPort(3010);

        client.onConnected(() -> System.out.println("Connected to server"));
        client.onDisconnected(() -> System.out.println("Disconnected from server"));
        client.onError(e -> System.err.println("Error: " + e));

        client.on("onChat", data -> System.out.println("Chat: " + data));
        client.on("onPlayerJoin", data -> System.out.println("PlayerJoin: " + data));

        client.connect("localhost", 3010);
        System.out.println("Client started");

        client.registerRoute("connector.entry", 1);
        client.registerRoute("player.move", 2);

        Object resp = client.requestSync("connector.entry", new Object[]{"Player1"});
        System.out.println("Response: " + resp);

        client.notify("player.move", new Object[]{100, 200});

        Thread.sleep(10000);
        client.disconnect();
    }

    public static void main(String[] args) throws Exception {
        new GameClient().start();
    }
}
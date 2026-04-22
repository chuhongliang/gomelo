package main

import (
	"fmt"
	"log"
	"time"

	"github.com/chuhongliang/gomelo/client/go"
)

func main() {
	client := go.NewClient(go.ClientOptions{
		Host:                 "localhost",
		Port:                 3010,
		Timeout:              5 * time.Second,
		HeartbeatInterval:    30 * time.Second,
		ReconnectInterval:    3 * time.Second,
		MaxReconnectAttempts: 5,
	})

	client.OnConnected(func() {
		fmt.Println("Connected to server")
	})

	client.OnDisconnected(func() {
		fmt.Println("Disconnected from server")
	})

	client.OnError(func(err error) {
		fmt.Printf("Error: %v\n", err)
	})

	client.On("onChat", func(data interface{}) {
		fmt.Printf("Chat received: %v\n", data)
	})

	client.On("onPlayerJoin", func(data interface{}) {
		fmt.Printf("Player joined: %v\n", data)
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Disconnect()

	client.RegisterRoute("connector.entry", 1)
	client.RegisterRoute("player.move", 2)

	resp, err := client.Request("connector.entry", map[string]interface{}{
		"name": "Player1",
	})
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
	} else {
		fmt.Printf("Response: %v\n", resp)
	}

	client.Notify("player.move", map[string]interface{}{
		"x": 100,
		"y": 200,
	})

	time.Sleep(10 * time.Second)
}
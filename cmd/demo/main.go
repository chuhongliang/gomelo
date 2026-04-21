package main

import (
	"encoding/json"
	"fmt"
	"github.com/chuhongliang/gomelo"
	"github.com/chuhongliang/gomelo/connector"
	"github.com/chuhongliang/gomelo/lib"
	"log"
)

func main() {
	app := gomelo.NewApp(
		gomelo.WithHost("0.0.0.0"),
		gomelo.WithPort(3010),
	)

	srv := connector.NewServer(&connector.ServerOptions{
		Host: "0.0.0.0",
		Port: 3010,
	})

	srv.OnConnect(func(session *lib.Session) {
		fmt.Printf("New connection: %d\n", session.ID())
	})

	srv.Handle("connector.entry", func(session *lib.Session, msg *lib.Message) (any, error) {
		var req struct {
			Token string `json:"token"`
		}
		if data, ok := msg.Body.([]byte); ok {
			json.Unmarshal(data, &req)
			fmt.Printf("Received: token=%s\n", req.Token)
		}

		session.Set("user", "test")

		return map[string]any{
			"code": 0,
			"msg":  "ok",
		}, nil
	})

	app.Register("connector", srv)

	log.Println("Starting server on :3010...")
	app.Start(func(err error) {
		if err != nil {
			log.Fatal(err)
		}
	})

	app.Wait()
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var PS_PORT = "6379"
var APP_PORT = "4200"

const (
	channel_name = "pubsub"
)

var rdb *redis.Client
var sub *redis.PubSub

type ChatRequest struct {
	Username  string `json:"username"`
	Message string `json:"message"`
}

var clients = make(map[*websocket.Conn]string)

func chat_handler() {
	for {
		data, err := sub.ReceiveMessage(context.Background())
		if err != nil {
			fmt.Printf("Warning: Failed to get message: %v", err.Error())
			continue
		}

		var msg ChatRequest
		err = json.Unmarshal([]byte(data.Payload), &msg)
		if err != nil {
			fmt.Printf("Could not unmarshal WS payload: %v", err.Error())
			continue
		}

		for ws, username := range clients {
			if username != msg.Username {
				writer, err := ws.NextWriter(1)
				if err != nil {
					fmt.Printf("Failed to get a writer for WS: %v", err.Error())
					continue
				}

				_, err = writer.Write([]byte(data.Payload))
				if err != nil {
					fmt.Printf("Failed to write message to WS: %v", err.Error())
					continue
				}

				writer.Close()
			}
		}
	}
}

func websocket_handler(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("X-USERNAME")
	if username == "" {
		fmt.Printf("No username header\n")
		w.WriteHeader(400)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to connect to WS: %v", err.Error())
		return
	}

	defer ws.Close()
	defer delete(clients, ws)

	clients[ws] = username

	for {
		// TODO: Authenticate WS write request
		_, reader, err := ws.NextReader()
		if err != nil {
			fmt.Printf("Failed to get a reader for WS: %v", err.Error())
			continue
		}

		p, err := io.ReadAll(reader)
		if err != nil {
			fmt.Printf("Failed to read message from WS: %v", err.Error())
			continue
		}

		var msg string = string(p)
		log.Printf("Message: %s\n", msg)
		rdb.Publish(context.Background(), channel_name, msg)
	}
}

func main() {
	if os.Getenv("PS_PORT") != "" {
		PS_PORT = os.Getenv("PS_PORT")
	}

	if os.Getenv("APP_PORT") != "" {
		APP_PORT = os.Getenv("APP_PORT")
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     "pubsub-service:" + PS_PORT,
		Password: "",
		DB:       0,
		Protocol: 2,
	})

	ctx := context.Background()
	log.Print(rdb.Ping(ctx))

	sub = rdb.Subscribe(ctx, channel_name)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/chat", websocket_handler)
	server := http.Server{
		Addr:    ":" + APP_PORT,
		Handler: mux,
	}

	go chat_handler()

	log.Printf("Server listening on port %s", APP_PORT)
	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("Failed to start server: %v", err.Error())
	}
}

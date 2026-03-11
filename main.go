package main

import (
	"context"
	"io"
	"log"
	"net/http"

	daprc "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/service/common"
	daprs "github.com/dapr/go-sdk/service/http"
	"github.com/gorilla/websocket"
)

var sub = &common.Subscription{
	PubsubName: "pubsub",
	Topic:      "chat",
	Route:      "/chat",
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var clients = make(map[*websocket.Conn]bool)

const (
	pub_name  = "pub_chat"
	pub_topic = "chat"
)

type test struct {
	pub daprc.Client
	ws  *websocket.Conn
}

func (_t test) upgrade_handler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal("Failed to upgrade connection to WS")
	}
	defer ws.Close()
	clients[ws] = true

	for {
		mt, reader, err := ws.NextReader() // blocking call
		if err != nil {
			log.Fatal(err.Error())
			return
		}
		if mt != 1 {
			continue
		}

		// TODO: Authenticate Request

		p, err := io.ReadAll(reader)
		if err != nil {
			log.Fatal("Failed to read from WS")
		}

		if err = _t.pub.PublishEvent(
			context.Background(),
			pub_name,
			pub_topic,
			p); err != nil {
			log.Fatal("Failed to publish event after receiving from WS")
		}
	}
}

func (_t test) topic_chat_handler(
	ctx context.Context,
	e *common.TopicEvent,
) (retry bool, err error) {
	/*	--------------- Ball Knowledge ------------------
		if _t.ws == nil {
			return false, errors.New("Nil pointer for WS connection")
		}
		writer, err := _t.ws.NextWriter(1) // message type = 1, get from 'e'
		if err != nil {
			log.Fatal("Could not get a writer for WS")
			return false, nil
		}
		defer writer.Close()

		_, err = writer.Write([]byte(e.Data.(string)))
		if err != nil {
			log.Fatal("Could not write to WS")
		}
	*/

	for ws, is_con := range clients {
		if ws != nil && is_con {
			writer, err := ws.NextWriter(1) // message type = 1, get form 'e'
			if err != nil {
				log.Fatal("Failed to get a writer for WS")
				writer.Close()
				continue
			}

			data := e.Data.(string)
			_, err = writer.Write([]byte(data))
			if err != nil {
				log.Fatal("Failed to write to WS")
			}

			writer.Close()
		}
	}

	return false, nil
}

func main() {
	c, err := daprc.NewClient()
	if err != nil {
		log.Fatal("Failed to create publisher client. Not starting the server.")
		return
	}
	var _t = test{
		pub: c,
	}
	http.HandleFunc("/ws", _t.upgrade_handler)
	server := daprs.NewServiceWithMux(":6969", nil)
	server.AddTopicEventHandler(sub, _t.topic_chat_handler)
	server.Start()
}

Systems:

I wanna support a few things. First of all:

Servers
    Channels --> two types
    - text channel
    - voice / video channel

Flow of data:
Client sends an HTTP request to https://discord.com/
Request contains info:
    - Server ID
    - Channel ID
    - Payload

MASTER PLAN:
```go
package main

var pub interface{} // one publisher per pod
var clients = make(map[conn]bool) // multiple subscibers per pod

func client(pub interface{}) {
	ws, err := websocket()
	sub := pub.Subscribe() // this is a blocking operation

	clients[ws] = true

	for {
		// received this through http websocket
		if ws.ReceiveMessage {
			evnt := createEvent()
			pub.PublishEvent(evnt)
		}

		if sub.ReceiveEvent {
			for client, connected := range clients {
				if (ws != client) && connected {
					msg := constructMessage(sub.data)
					ws.WriteMessage(msg)
				}
			}
		}
	}
}

const Client struct {
    Pub: pub,
    Sub: sub,
}

func handler() {
    ws, err := websocket()
    for {

    }
}
```

every subsciber is also a producer
```go
func WsHandler() {
    ws, err := websocket()

    func handler(e event) {
        ws.MessageSend(e.Data)
    }

    sub = AddTop(handler) // subscriber server
    go sub.Start() // spawn new goroutine

    for {
        if ws.MessageRecv() {
            // publish it
        }
    }
}
```

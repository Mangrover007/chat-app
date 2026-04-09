package websocket

import (
	"log"
	"net/http"
	"time"

	"github.com/Mangrover007/discord-clone-2/app/internal/service"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,
}

type WS_Handler struct {
	wss *service.WS_Service
}

func NewHandler(wss *service.WS_Service) *WS_Handler {
	return &WS_Handler{
		wss: wss,
	}
}

func (wsh *WS_Handler) UpgradeConn(w http.ResponseWriter, r *http.Request) {
	user_id := r.Header.Get("x-uid")

	log.Print("NEW CONNECTION: ", user_id)

	ws, err := upgrader.Upgrade(w, r, nil)
	defer func(){
		wsh.wss.Remove_User(user_id)
		ws.Close()
	}()
	if err != nil {
		log.Print("ERROR: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// ws.SetPongHandler(func(appData string) error {
	// 	log.Print("PONG RECEIVED")
	// 	return nil
	// })

	// ws.SetPingHandler(func(appData string) error {
	// 	log.Print("RECEIVED PING")
	// 	return ws.WriteMessage(websocket.PongMessage, nil)
	// })

	wc := make(chan[]byte, 2000)
	wsh.wss.Add_User(user_id, wc)
	log.Print("WS ADDED FOR USER: ", user_id)


	timer := time.NewTicker(5 * time.Second)
	defer timer.Stop()
	go func(){
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				log.Print("Error (websockets.handlers.go): %s line %d", err.Error(), 64)
				return
			}
		}
	}()
	for {
		select {
		// case <-wc:
		// 	log.Print("BYE")
		// 	return
		case c := <-wc:
			err := ws.WriteMessage(1, c)
			if err != nil {
				log.Printf("ERROR (websockets.handlers.go): %s line %d", err.Error(), 77)
				return
			}
		case <-timer.C:
			// log.Print("SENDING PING: ", t)
			err := ws.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				log.Print("ERROR (websockets.handlers.go): %s line %d", err.Error(), 84)
				return
			}
		// default:
		// 	log.Print("CHANNEL FULL", user_id)
		// 	continue
		}
	}
}

func (wsh *WS_Handler) Change_Guild(w http.ResponseWriter, r *http.Request) {
	user_id := r.Header.Get("x-uid")
	guild_id := r.PathValue("guild_id")
	wsh.wss.Change_Guild(user_id, guild_id)
	w.WriteHeader(200)
}

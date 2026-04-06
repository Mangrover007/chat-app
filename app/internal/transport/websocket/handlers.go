package websocket

import (
	"log"
	"net/http"

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
	if err != nil {
		log.Print("ERROR: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	wsh.wss.Add_User(user_id, ws)
	log.Print("WS ADDED FOR USER: ", user_id)
}

func (wsh *WS_Handler) Change_Guild(w http.ResponseWriter, r *http.Request) {
	user_id := r.Header.Get("x-uid")
	guild_id := r.PathValue("guild_id")
	wsh.wss.Change_Guild(user_id, guild_id)
	w.WriteHeader(200)
}

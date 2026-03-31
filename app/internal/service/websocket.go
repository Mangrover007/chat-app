package service

import (
	"context"
	"log"

	"github.com/Mangrover007/discord-clone-2/app/internal/state"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type WS_Service struct {
	server_id string
	rdb       *redis.Client
	cp        *state.Conn_Pool
}

func NewWebsocketService(server_id string, rdb *redis.Client, cp *state.Conn_Pool) *WS_Service {
	return &WS_Service{
		server_id: server_id,
		rdb:       rdb,
		cp:        cp,
	}
}

// TODO: user_id to UUID
func (wss *WS_Service) Add_User(user_id string, ws *websocket.Conn) {
	_, err := wss.rdb.Set(context.Background(), "user:"+user_id, wss.server_id, 0).Result()
	if err != nil {
		log.Print("ERROR: ", err.Error())
		return
	}

	wss.cp.Add_Conn(user_id, ws)
}

func (wss *WS_Service) Change_Guild(user_id, guild_id string) {
	wss.cp.Change_Guild(user_id, guild_id)
}

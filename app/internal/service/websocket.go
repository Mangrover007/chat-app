package service

import (
	"context"
	"log"

	"github.com/Mangrover007/discord-clone-2/app/internal/state"
	// "github.com/gorilla/websocket"
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
func (wss *WS_Service) Add_User(user_id string, wc chan[]byte) {
	// log.Print("ADDING USER TO REDIS STORE: ", user_id)
	_, err := wss.rdb.Set(
		context.Background(), 
		"user:" + user_id,
		wss.server_id,
		0,
	).Result()
	if err != nil {
		log.Print("ERROR: ", err.Error())
		return
	}

	// log.Print("ADDED USER TO REDIS STORE, ADDING TO INTERNAL CONNECTION POOL: ", user_id)
	wss.cp.Add_Conn(user_id, wc)
	// log.Print("ADDED USER TO INTERNAL CONNECTION POOL: ", user_id)
}

func (wss *WS_Service) Remove_User(user_id string) {
	wss.cp.Remove_Conn(user_id)
}

// user --> pod mapping = user_id pod_id							MAP 1
// user --> guild mapping = user_id:guild guild_id					MAP 2
// guild (set) --> user mapping = guild:guild_id user_id:server_id	MAP 3
func (wss *WS_Service) Change_Guild(user_id, guild_id string) {
	// from MAP 1
	serv_id, err := wss.rdb.Get(
		context.Background(),
		"user:" + user_id,
	).Result()
	if err != nil {
		// This means user is not connected through websocket. In this case,
		// return immediately.
		log.Printf("ERROR (websocket.go): %s on line %d", err.Error(), 57)
		log.Print("Received user ID: ", user_id)
		return
	}

	// from MAP 2
	prev_guild, err := wss.rdb.Get(
		context.Background(),
		user_id + ":guild",
	).Result()
	if err != nil {
		// this means user was not in a guild previously
		// ignore this error basically
		if err != redis.Nil {
			log.Printf("ERROR (websocket.go): %s on line %d", err.Error(), 71)
			return
		}
	}

	_, err = wss.rdb.SRem(
		context.Background(),
		"guild:" + prev_guild,
		user_id + ":" + serv_id,
	).Result()
	if err != nil {
		log.Printf("ERROR (websocket.go): %s on line %d", err.Error(), 82)
		return
	}

	_, err = wss.rdb.SAdd(
		context.Background(),
		"guild:" + guild_id,
		user_id + ":" + serv_id,
	).Result()
	if err != nil {
		log.Printf("ERROR (websocket.go): %s on line %d", err.Error(), 92)
		return
	}

	// wss.cp.Change_Guild(user_id, guild_id)
}

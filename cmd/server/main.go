package main

import (
	"context"
	"log"
	"net/http"

	"github.com/Mangrover007/discordv2/internal/state"
	"github.com/Mangrover007/discordv2/internal/stream"
	api "github.com/Mangrover007/discordv2/internal/transport/http"
	ws "github.com/Mangrover007/discordv2/internal/transport/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func main() {

	server_id := "stream:s1"
	cp := state.NewConnPool()

	// -------------------- DB CONNECTION -------------------------
	psql, err := pgx.Connect(context.Background(), "postgres://postgres:mango@127.0.0.1:5432/discordv2")
	if err != nil {
		log.Fatal("Server crashed")
		return
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB: 0,
		Protocol: 2,
		Password: "",
	})
	_ = rdb.XGroupCreateMkStream(context.Background(), server_id, "group:g1", "$").Err()
	// if err != nil {
	// 	log.Fatal("ERROR: Could not make stream: ", err.Error())
	// 	return
	// }

	// ------------------------------------------------------------

	go stream.Consumer(rdb, server_id, "group:g1", cp)

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api.NewRouter(psql, rdb)))
	mux.Handle("/ws/", http.StripPrefix("/ws", ws.NewRouter(server_id, rdb, cp)))

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Print("Server started, listening on port :8080")
	log.Print("Pod ID: ", server_id)
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal("FATAL: server crashed", err.Error())
	}
}

package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/Mangrover007/discord-clone-2/app/internal/state"
	"github.com/Mangrover007/discord-clone-2/app/internal/stream"
	api "github.com/Mangrover007/discord-clone-2/app/internal/transport/http"
	ws "github.com/Mangrover007/discord-clone-2/app/internal/transport/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {

	server_id := "server:" + os.Getenv("POD_UID")
	if server_id == "" {
		server_id = "server:s1"
	}
	cp := state.NewConnPool()

	// -------------------- DB CONNECTION -------------------------
	psql, err := pgxpool.New(context.Background(), os.Getenv("DB_URL"))
	if err != nil {
		log.Print("Server crashed", err.Error())
		return
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_URL"),
		DB: 0,
		Protocol: 2,
		Password: os.Getenv("REDIS_PASSWORD"),
	})
	_ = rdb.XGroupCreateMkStream(context.Background(), server_id, "group:g1", "$").Err()
	if err != nil {
		log.Print("ERROR: Could not make stream: ", err.Error())
		return
	}

	// ------------------------------------------------------------

	go stream.Consumer(rdb, server_id, "group:g1", cp)

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api.NewRouter(psql, rdb)))
	mux.Handle("/ws/", http.StripPrefix("/ws", ws.NewRouter(server_id, rdb, cp)))

	server := http.Server{
		Addr:    ":" + os.Getenv("SERVER_PORT"),
		Handler: mux,
	}

	log.Print("Server started, listening on port " + os.Getenv("SERVER_PORT"))
	log.Print("Pod ID: ", server_id)
	err = server.ListenAndServe()
	if err != nil {
		log.Print("FATAL: server crashed", err.Error())
	}
}

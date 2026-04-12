package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/Mangrover007/discord-clone-2/app/internal/state"
	"github.com/Mangrover007/discord-clone-2/app/internal/stream"
	api "github.com/Mangrover007/discord-clone-2/app/internal/transport/http"
	ws "github.com/Mangrover007/discord-clone-2/app/internal/transport/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {

	// ------------------- LOAD ENV VARIABLES -------------------
	var PSQL_URI = os.Getenv("DB_URL")
	if PSQL_URI == "" {
		PSQL_URI = "postgres://postgres:mango@127.0.0.1:5432/discordv2"
	}

	var REDIS_URI = os.Getenv("REDIS_URL")
	if REDIS_URI == "" {
		REDIS_URI = "127.0.0.1:6379"
	}

	var REDIS_PASSWORD = os.Getenv("REDIS_PASSWORD")

	var SERV_PORT = os.Getenv("SERVER_PORT")
	if SERV_PORT == "" {
		SERV_PORT = "8080"
	}

	// ------------------------------------------------------------

	server_id := os.Getenv("POD_UID")
	if server_id == "" {
		server_id = "s1"
	}
	cp := state.NewConnPool()

	// -------------------- DB CONNECTION -------------------------
	psql, err := pgxpool.New(context.Background(), PSQL_URI)
	if err != nil {
		log.Print("Server crashed", err.Error())
		return
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: REDIS_URI,
		DB: 0,
		Protocol: 2,
		Password: REDIS_PASSWORD,
	})

	// -------------------- REDIS STREAMS ------------------------
	_, err = rdb.XGroupCreateMkStream(
		context.Background(),
		"server:" + server_id,
		"group:g1",
		"$",
	).Result()
	if err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			log.Print("ERROR: Could not make stream: ", err.Error())
			return
		}
	}

	_, err = rdb.XGroupCreateMkStream(
		context.Background(),
		"db:write",
		"db:consumer:1",
		"$",
	).Result()
	if err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			log.Print("ERROR: Could not make stream: ", err.Error())
			return
		}
	}
	
	go stream.Msg_Consumer(rdb, server_id, "group:g1", cp)
	go stream.DB_Consumer(rdb, psql, "db:write", "db:consumer:1")

	// ------------------------------------------------------------

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api.NewRouter(psql, rdb)))
	mux.Handle("/ws/", http.StripPrefix("/ws", ws.NewRouter(server_id, rdb, cp)))

	server := http.Server{
		Addr:    ":" + SERV_PORT,
		Handler: mux,
	}

	log.Print("Server started, listening on port " + SERV_PORT)
	log.Print("Pod ID: ", server_id)
	err = server.ListenAndServe()
	if err != nil {
		log.Print("FATAL: server crashed", err.Error())
	}
}

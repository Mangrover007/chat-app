package http

import (
	"net/http"

	"github.com/Mangrover007/discord-clone-2/app/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func NewRouter(psql *pgxpool.Pool, rdb *redis.Client) *http.ServeMux {

	ms := service.NewMessageService(psql, rdb)
	mh := NewMessageHandler(ms)

	mux := http.NewServeMux()

	mux.HandleFunc("/{guild_id}/{channel_id}", mh.Msg_handler)
	mux.HandleFunc("/register", mh.Register_Handler)
	mux.HandleFunc("/{guild_id}", mh.Guild_Join_Handler)

	return mux
}

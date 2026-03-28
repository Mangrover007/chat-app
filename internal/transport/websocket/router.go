package websocket

import (
	"net/http"

	"github.com/Mangrover007/discordv2/internal/service"
	"github.com/Mangrover007/discordv2/internal/state"
	"github.com/redis/go-redis/v9"
)

func NewRouter(server_id string, rdb *redis.Client, cp *state.Conn_Pool) *http.ServeMux {

	ws_handler := NewHandler(service.NewWebsocketService(server_id, rdb, cp))

	mux := http.NewServeMux()

	mux.HandleFunc("/", ws_handler.UpgradeConn)
	mux.HandleFunc("/{guild_id}/{channel_id}", ws_handler.Change_Guild)

	return mux
}

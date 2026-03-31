package http

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/Mangrover007/discord-clone-2/app/internal/service"
	"github.com/Mangrover007/discord-clone-2/app/internal/domain"
)

type MessageHandler struct {
	ms *service.MessageService
}

func NewMessageHandler(ms *service.MessageService) *MessageHandler {
	return &MessageHandler{
		ms: ms,
	}
}

func (mh *MessageHandler) Msg_handler(w http.ResponseWriter, r *http.Request) {
	guild_id := r.PathValue("guild_id")
	channel_id := r.PathValue("channel_id")

	p, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var payload domain.Payload
	err = json.Unmarshal(p, &payload)

	mh.ms.Send_msg(&domain.Message{
		Content:  payload.Content,
		UserID:   r.Header.Get("X-User-ID"),
		Guild:    guild_id,
		Channel:  channel_id,
	})

	w.WriteHeader(http.StatusOK)
}

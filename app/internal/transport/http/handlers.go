package http

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Mangrover007/discord-clone-2/app/internal/domain"
	"github.com/Mangrover007/discord-clone-2/app/internal/service"
)

// The purpose of this file is to extract information
// from the HTTP requests, and give relevant information
// to the service layer

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
	uid := r.Header.Get("x-uid")

	p, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var payload domain.Payload
	err = json.Unmarshal(p, &payload)

	if err != nil {
		log.Print("ERROR (http.handlers.go): ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)

	timestamp := time.Now().UTC()

	// Add message to Redis stream belonging to this Pod
	mh.ms.Send_msg(&domain.Message{
		Content:  payload.Content,
		UserID:   r.Header.Get("x-uid"),
		Guild:    guild_id,
		Channel:  channel_id,
		Timestamp: timestamp,
	})

	if err != nil {
		log.Print("ERROR (handlers.go): ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"Content": payload.Content,
		"Sender": uid,
		"Timestamp": timestamp,
	})
}

func (mh *MessageHandler) Register_Handler(w http.ResponseWriter, r *http.Request) {
	var payload domain.RegisterPayload
	err := json.NewDecoder(r.Body).Decode(&payload)

	uid, err := mh.ms.Register_user(payload.Username, payload.Password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"id": uid,
	})
}

func (mh *MessageHandler) Guild_Join_Handler(w http.ResponseWriter, r *http.Request) {
	guild_id := r.PathValue("guild_id")
	uid := r.Header.Get("x-uid")

	err := mh.ms.Guild_Join(uid, guild_id)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

package domain

import (
	"time"

	"github.com/redis/go-redis/v9"
)

type Payload struct {
	Content  string `json:"content"`
}

type RegisterPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Message struct {
	Content   string
	UserID    string
	Guild     string // TODO: UUID later
	Channel   string // TODO: UUID later
	Timestamp time.Time
}

type BroadcastTask struct {
	Members []string
	Msg     redis.XMessage
}

type DB_Msg struct {
	Sender_id    string
	Channel_id   string
	Text_content string
	Created_at   string
}

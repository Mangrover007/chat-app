package domain

import "time"

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

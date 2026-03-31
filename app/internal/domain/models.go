package domain

type Payload struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

type Message struct {
	Content  string
	UserID   string
	Guild    string // TODO: UUID later
	Channel  string // TODO: UUID later
}

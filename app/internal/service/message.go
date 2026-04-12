package service

import (
	"context"
	"log"
	"strings"

	"github.com/Mangrover007/discord-clone-2/app/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// This file contains mostly SQL queries.

// It contains all business logic related to transport.http
// It is the service layer of transport.http, followint the
// the controller --> service model

type MessageService struct {
	psql *pgxpool.Pool
	rdb  *redis.Client
}

func NewMessageService(psql *pgxpool.Pool, rdb *redis.Client) *MessageService {
	return &MessageService{
		psql: psql,
		rdb:  rdb,
	}
}

// This function pushes the message to all relevant PODs's queues, and also
// pushes the message to the DB write queue.
// NOTE:
// Redis tracks the set of members in a guild in the format:
// guild:<guild_id> --> set of <user_id>:<server_id>
func (ms *MessageService) Send_msg(msg *domain.Message) {
	
	// Push to all relevant PODs's queues
	members, err := ms.rdb.SMembers(
		context.Background(),
		"guild:" + msg.Guild,
	).Result()
	if err != nil {
		log.Printf("ERROR (message.go): %s on line %d", err.Error(), 47)
	}

	servers := make(map[string]bool) // aka POD IDs
	for _, member := range members {
		val := strings.Split(member, ":")
		servers[val[1]] = true
	}

	for server_id, _ := range servers {
		ms.rdb.XAdd(context.Background(), &redis.XAddArgs{
			Stream: "server:" + server_id,
			Values: map[string]interface{}{
				"Content":   msg.Content,
				"UserID":    msg.UserID,
				"Guild":     msg.Guild,
				"Channel":   msg.Channel,
				"Timestamp": msg.Timestamp,
			},
		})

		// TODO: check if timestamp is standard or not
		// The reason I say to check, is because if I let DB decide,
		// then I can't decouple DB and backend.
		// otherwise they would report different timestamp of the same message
	}

	// push to DB write queue
	_, err = ms.rdb.XAdd(
		context.Background(),
		&redis.XAddArgs{
			Stream: "db:write",
			NoMkStream: true,
			Values: map[string]interface{}{
				"sender_id":    msg.UserID,
				"channel_id":   msg.Channel,
				"text_content": msg.Content,
				"created_at":   msg.Timestamp,
			},
		},
	).Result()
	if err != nil {
		log.Printf("ERROR (messages.go): %s line: %d", err.Error(), 118)
		return
	}
}

func (ms *MessageService) Register_user(username, password string) (uid string, err error) {
	res := ms.psql.QueryRow(context.Background(), `
		INSERT INTO users (username, password, created_at)
		VALUES ($1, $2, NOW())
		RETURNING id`,
		username, password,
	)

	err = res.Scan(&uid)
	if err != nil {
		log.Printf("ERROR (messages.go): %s line: %d", err.Error(), 128)
		return "", err
	}

	return uid, err
}

func (ms *MessageService) Guild_Join(uid, guild_id string) error {
	_, err := ms.psql.Exec(context.Background(), `
		INSERT INTO guild_members (member_id, guild_id)
		VALUES ($1, $2)`,
		uid, guild_id,
	)

	if err != nil {
		log.Printf("ERROR (messages.go): %s line: %d", err.Error(), 143)
	}

	return err
}

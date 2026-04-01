package service

import (
	"context"
	"log"

	"github.com/Mangrover007/discord-clone-2/app/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// This file contains mostly SQL queries.

// It contains all business logic related to transport.http
// It is the service layer of transport.http, pertaining to
// the controller --> service model

type MessageService struct {
	psql *pgx.Conn
	rdb  *redis.Client
}

func NewMessageService(psql *pgx.Conn, rdb *redis.Client) *MessageService {
	return &MessageService{
		psql: psql,
		rdb:  rdb,
	}
}

func (ms *MessageService) Send_msg(msg *domain.Message) {
	res, err := ms.psql.Query(
		context.Background(),
		"SELECT member_id FROM guild_user WHERE guild_id = $1",
		msg.Guild,
	)
	if err != nil {
		log.Print("ERROR (messages.go): ", err.Error())
		return
	}

	servers := make(map[string]bool)
	for {
		if !res.Next() {
			break
		}

		var user_id string
		err := res.Scan(&user_id)
		if err != nil {
			log.Print("ERROR (messages.go): ", err.Error())
			return
		}

		server_id, err := ms.rdb.Get(context.Background(), "user:"+user_id).Result()
		if err != nil {
			log.Print("ERROR (messages.go): ", err.Error())
			return
		}

		servers[server_id] = true
	}

	for server, _ := range servers {
		ms.rdb.XAdd(context.Background(), &redis.XAddArgs{
			Stream: server,
			Values: map[string]interface{}{
				"Content":  msg.Content,
				"UserID":   msg.UserID,
				"Guild":    msg.Guild,
				"Channel":  msg.Channel,
			},
		})
	}
}

func (ms *MessageService) Register_user(username, password string) (uid string, err error) {
	res, err := ms.psql.Query(context.Background(), `
		INSERT INTO users (username, password)
		VALUES ($1, $2)
		RETURNING id`,
		username, password,
	)

	if err != nil {
		log.Print("ERROR (messages.go): ", err.Error())
	}

	if res.Next() {
		err = res.Scan(&uid)
		if err != nil {
			log.Print("ERROR: ", err.Error())
		}
		res.Close()
	}

	return uid, err
}

func (ms *MessageService) Guild_Join(uid, guild_id string) error {
	_, err := ms.psql.Exec(context.Background(), `
		INSERT INTO guild_user (member_id, guild_id)
		VALUES ($1, $2)`,
		uid, guild_id,
	)

	if err != nil {
		log.Print("ERROR (messages.go): ", err.Error())
	}

	return err
}

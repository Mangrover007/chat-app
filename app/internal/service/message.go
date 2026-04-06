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
// It is the service layer of transport.http, pertaining to
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

func (ms *MessageService) Send_msg(msg *domain.Message) {
	// res, err := ms.psql.Query(
	// 	context.Background(),
	// 	"SELECT member_id FROM guild_user WHERE guild_id = $1",
	// 	msg.Guild,
	// )
	// if err != nil {
	// 	log.Printf("ERROR (messages.go): %s line: %d", err.Error(), 38)
	// 	return
	// }

	members, err := ms.rdb.SMembers(
		context.Background(),
		"guild:" + msg.Guild,
	).Result()
	if err != nil {
		log.Printf("ERROR (message.go): %s on line %d", err.Error(), 47)
	}

	// log.Print("DB Query Completed")

	servers := make(map[string]bool)
	for _, member := range members {
		val := strings.Split(member, ":")
		servers[val[1]] = true
	}
	// log.Printf("SERVERS: %+v", servers)
	// for {
	// 	if !res.Next() {
	// 		break
	// 	}

	// 	var user_id string
	// 	err := res.Scan(&user_id)
	// 	if err != nil {
	// 		log.Printf("ERROR (messages.go): %s line: %d", err.Error(), 66)
	// 		return
	// 	}

	// 	server_id, err := ms.rdb.Get(context.Background(), "user:"+user_id).Result()
	// 	if err != nil {
	// 		if err == redis.Nil {
	// 			// since when seeing the logs, I will know which pod im looking at
	// 			// i dont need to provide Pod's ID
	// 			// log.Printf("INFO: user %s is not connected to this Pod", user_id)
	// 			continue
	// 		}
	// 		log.Printf("ERROR (messages.go): %s line: %d", err.Error(), 78)
	// 		res.Close()
	// 		return
	// 	}

	// 	servers[server_id] = true
	// }

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
}

func (ms *MessageService) Register_user(username, password string) (uid string, err error) {
	res := ms.psql.QueryRow(context.Background(), `
		INSERT INTO users (username, password)
		VALUES ($1, $2)
		RETURNING id`,
		username, password,
	)

	// if err != nil {
	// 	log.Printf("ERROR (messages.go): %s line: %d", err.Error(), 114)
	// 	return "", err
	// }

	// if res.Next() {
	// 	err = res.Scan(&uid)
	// 	if err != nil {
	// 		log.Print("ERROR: ", err.Error())

	// 	}
	// }

	err = res.Scan(&uid)
	if err != nil {
		log.Printf("ERROR (messages.go): %s line: %d", err.Error(), 128)
		return "", err
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
		log.Printf("ERROR (messages.go): %s line: %d", err.Error(), 143)
	}

	return err
}

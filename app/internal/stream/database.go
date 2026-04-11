package stream

import (
	"context"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func init() {
	return
}

func DB_Consumer(rdb *redis.Client, psql *pgxpool.Pool) {

	var backlog = true

	for {
		myid := ">"
		if backlog {
			myid = "0-0"
		}

		res, err := rdb.XReadGroup(
			context.Background(),
			&redis.XReadGroupArgs{
				Group:   "db:consumer:1",
				Streams: []string{"db:write", myid},
				Count:   100000,
				Block:   0,
			},
		).Result()
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "key") || strings.Contains(err.Error(), "no such") {
				continue
			}
			log.Print("ERROR (stream.consumer.go): ", err.Error())
			continue
		}

		if len(res[0].Messages) == 0 {
			backlog = false
			continue
		}

		for _, msg := range res[0].Messages {
			_, err := psql.Exec(
				context.Background(),
				`INSERT INTO channel_messages (sender_id, channel_id, text_content, created_at)
				VALUES ($1, $2, $3, $4)`,
				msg.Values["sender_id"],
				msg.Values["channel_id"],
				msg.Values["text_content"],
				msg.Values["created_at"],
			)
			if err != nil {
				log.Print("ERROR (stream.consumer.go): ", err.Error())
				log.Fatal("FATAL ERROR")
				return
			}

			rdb.XAck(
				context.Background(),
				"db:writer",
				"db:consumer:1",
				msg.ID,
			)
		}
	}
}

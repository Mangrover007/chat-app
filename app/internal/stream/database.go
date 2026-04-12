package stream

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/Mangrover007/discord-clone-2/app/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func DB_Consumer(rdb *redis.Client, psql *pgxpool.Pool, stream_id, group_id string) {
	// This channel is used by db_consumer to put new messages it receives from
	// the Redis stream, and by db_worker to write them to the database.
	db_chan := make(chan domain.DB_Msg, 100000)
	go db_consumer(rdb, psql, stream_id, group_id, db_chan)
	for i := 0; i < 4; i++ {
		go db_worker(db_chan, psql, i)
	}
}

// NOTE: not reading from the db_chan channel directly because
// in the future, this service will be running separately
// AKA, in the name of scalability
func db_consumer(rdb *redis.Client, psql *pgxpool.Pool, stream_id, group_id string, db_chan chan domain.DB_Msg) {
	var backlog = true
	var count int64 = 100000
	for {
		myid := ">"
		if backlog {
			myid = "0-0"
		}

		res, err := rdb.XReadGroup(
			context.Background(),
			&redis.XReadGroupArgs{
				Group:   group_id,
				Streams: []string{stream_id, myid},
				Count:   count,
				Block:   0,
				Consumer: "db:consumer:1",
			},
		).Result()
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "key") {
				continue
			}
			log.Print("ERROR (stream.database.go): ", err.Error())
			continue
		}
		if len(res[0].Messages) == 0 {
			backlog = false
			continue
		}

		for _, msg := range res[0].Messages {
			// log.Printf("NEW DB WRITE: %+v", msg.Values)
			db_chan <- domain.DB_Msg{
				Sender_id:    msg.Values["sender_id"].(string),
				Channel_id:   msg.Values["channel_id"].(string),
				Text_content: msg.Values["text_content"].(string),
				Created_at:   msg.Values["created_at"].(string),
			}

			rdb.XAck(
				context.Background(),
				stream_id,
				group_id,
				msg.ID,
			)
		}
	}
}

// This function writes to the DB every 5 seconds. It is timer based instead of
// len(db_chan) based because doing the latter would ony write to the DB when
// the NEXT message comes in.
func db_worker(db_chan chan domain.DB_Msg, psql *pgxpool.Pool, worker_id int) {
	rows := make([][]interface{}, 0, 4000)
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case new_msg := <-db_chan:
			rows = append(rows, []interface{}{
				new_msg.Sender_id,
				new_msg.Channel_id,
				new_msg.Text_content,
				new_msg.Created_at,
			})

		case <-ticker.C:
			_, err := psql.CopyFrom(
				context.Background(),
				pgx.Identifier{"channel_messages"},
				[]string{"sender_id", "channel_id", "text_content", "created_at"},
				pgx.CopyFromRows(rows),
			)
			if err != nil {
				log.Printf("ERROR (stream.database.go): %s line %d", err.Error(), 102)
				log.Print("Trying to insert rows one by one...")
				for _, row := range rows {
					_, err := psql.Exec(
						context.Background(),
						`INSERT INTO channel_messages (sender_id, channel_id, text_content, created_at)
						VALUES ($1, $2, $3, $4)`,
						row[0], row[1], row[2], row[3],
					)
					if err != nil {
						log.Printf("ERROR (stream.database.go): %s line %d", err.Error(), 112)
					}
				}
			}
			rows = rows[:0]
		}
	}
}

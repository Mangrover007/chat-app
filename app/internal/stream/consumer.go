package stream

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/Mangrover007/discord-clone-2/app/internal/state"
	"github.com/redis/go-redis/v9"
)

type BroadcastTask struct {
	Members []string
	Msg     redis.XMessage
}

func Msg_Consumer(rdb *redis.Client, stream, group string, cp *state.Conn_Pool) {
	comms := make(chan BroadcastTask, 100000)
	db_chan := make(chan BroadcastTask, 100000)

	go consumer(rdb, stream, group, cp, comms)
	for i := 0; i < 4; i++ {
		go worker(comms, cp)
	}

	for {
		select {
		case new_batch := <-db_chan:
			msg := new_batch.Msg
			rdb.XAdd(
				context.Background(),
				&redis.XAddArgs{
					Stream: "db:write",
					NoMkStream: true,
					Values: map[string]interface{}{
						"sender_id":    msg.Values["UserID"],
						"channel_id":   "temp",
						"text_content": msg.Values["Content"],
						"created_at":   msg.Values["Timestamp"],
					},
				},
			)
		}
	}
}

func consumer(rdb *redis.Client, stream, group string, cp *state.Conn_Pool, comms chan BroadcastTask) {
	
	var backlog = true
	var count int64 = 100000

	for {
		var myid string
		if backlog {
			myid = "0-0"
		} else {
			myid = ">"
		}

		res, err := rdb.XReadGroup(context.Background(), &redis.XReadGroupArgs{
			Streams: []string{"server:" + stream, myid},
			Group: group,
			Block: 0,
			Consumer: "consumer:c1",
			Count: count,
		}).Result()

		// log.Print("READ")

		if err != nil {
			log.Print("ERROR: ", err.Error())
			continue
		}

		// Values: map[string]interface{}{
		// 		"Content":    msg.Content,
		// 		"Username":   msg.Username,
		// 		"Guild":      msg.Guild,
		// 		"Channel":    msg.Channel,
		// 		"Timestamp":  time.Now(),
		// }

		if len(res[0].Messages) == 0 {
			backlog = false
			continue
		}
		
		// log.Printf("New Message: %+v", res[0].Messages)
		for _, msg := range res[0].Messages {
			guild_id := msg.Values["Guild"]
			members, err := rdb.SMembers(
				context.Background(),
				"guild:" + guild_id.(string),
			).Result()
			if err != nil {
				// log.Printf("ERROR (consumer.go): %s, on line %d", err.Error(), 62)
				continue
			}
			comms <- BroadcastTask{
				Members: members,
				Msg: msg,
			}
			
			rdb.XAck(context.Background(), "server:" + stream, group, msg.ID)
		}
		

		log.Printf("Delivered all %d messages.", len(res[0].Messages))
	}
}

func worker(comms chan BroadcastTask, cp *state.Conn_Pool) {
	for {
		select {
		case new_batch := <-comms:
			var users = make(map[string]bool)
			for _, member := range new_batch.Members {
				val := strings.Split(member, ":")
				users[val[0]] = true
			}
			// users := cp.Get_Users_From_Guild(guild_id.(string))
			
			// log.Printf("USER ID LIST: %+v", users)
				
			data, _ := json.Marshal(new_batch.Msg.Values)
			for user_id, _ := range users {
				// DONT skip this guy
				// if user_id == msg.Values["UserID"] {
				// 	continue
				// }

				wc := cp.Get_WS_Conn(user_id)
				
				if wc == nil {
					// log.Print("NIL WS: ", user_id)
					// log.Print("GETTING WS FOR USER_ID: ", user_id)
					// log.Printf("WS FOR USER_ID %s IS: %+v", user_id, ws)
					continue
				}

				// writer, err := ws.NextWriter(1)
				// if err != nil {
				// 	log.Printf("ERROR (consumer.go): %s line %d", err.Error(), 91)
				// 	// writer.Close()
				// 	continue
				// }
				// json.NewEncoder(writer).Encode(msg.Values)
				// writer.Close()

				// ------------------------------------ CHANNEL REPLACEMENT --------------------------------
				if len(wc) == cap(wc) {
					log.Print("CHANNEL FULL")
				}
				wc <- data
			}
		}
	}
}

/*
There are two problems in all of this:

1. how is a user getting queried for in connection pool when they havent logged on and connected

2. why im still getting a nil pointer dereference when the ws connection is not nil, on writer.Close() of all things

first, what is write: broken pipe error? as i understand it, it is related to connection pools but i have little idea about those.
*/

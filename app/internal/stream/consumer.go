package stream

import (
	"context"
	"log"

	"github.com/Mangrover007/discord-clone-2/app/internal/state"
	"github.com/redis/go-redis/v9"
)

func Consumer(rdb *redis.Client, stream string, group string, cp *state.Conn_Pool) {
	
	var backlog = true

	for {
		var myid string
		if backlog {
			myid = "0-0"
		} else {
			myid = ">"
		}

		res, err := rdb.XReadGroup(context.Background(), &redis.XReadGroupArgs{
			Streams: []string{stream, myid},
			Group: group,
			Block: 0,
			Consumer: "consumer:c1",
			Count: 10,
		}).Result()

		// log.Print("READ")

		if err != nil {
			log.Print("ERROR: ", err.Error())
			continue
		}

		// Values: map[string]interface{}{
		// 		"Content":  msg.Content,
		// 		"Username": msg.Username,
		// 		"Guild":    msg.Guild,
		// 		"Channel":  msg.Channel,
		// }

		if len(res[0].Messages) == 0 {
			backlog = false
			continue
		}

		log.Printf("New Message: %+v", res[0].Messages)
		
		for _, msg := range res[0].Messages {
			guild_id := msg.Values["Guild"]
			users := cp.Get_Users_From_Guild(guild_id.(string))
			
			log.Printf("USER ID LIST: %+v", users)
			
			for user_id, _ := range users {
				if user_id == msg.Values["UserID"] {
					continue
				}
				log.Print("SENDER, RECEIVER: ", msg.Values["UserID"], " ", user_id)
				ws := cp.Get_WS_Conn(user_id)
				writer, _ := ws.NextWriter(1)
				writer.Write([]byte(msg.Values["Content"].(string)))
				writer.Close()
			}

			rdb.XAck(context.Background(), stream, group, msg.ID)
		}
	}
}

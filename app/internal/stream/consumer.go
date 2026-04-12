package stream

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/Mangrover007/discord-clone-2/app/internal/domain"
	"github.com/Mangrover007/discord-clone-2/app/internal/state"
	"github.com/redis/go-redis/v9"
)


func Msg_Consumer(rdb *redis.Client, stream, group string, cp *state.Conn_Pool) {
	// This channel is used by message_consumer to put new messages it receives from
	// the Redis stream, and by message_worker to broadcast them to relevant users.
	comms := make(chan domain.BroadcastTask, 100000)
	go msg_consumer(rdb, stream, group, cp, comms)
	for i := 0; i < 4; i++ {
		go msg_worker(comms, cp, i)
	}
}

func msg_consumer(rdb *redis.Client, stream, group string, cp *state.Conn_Pool, comms chan domain.BroadcastTask) {
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
			Consumer: "msg:consumer:c1",
			Count: count,
		}).Result()
		if err != nil {
			log.Print("ERROR: ", err.Error())
			continue
		}
		if len(res[0].Messages) == 0 {
			backlog = false
			continue
		}
		
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
			comms <- domain.BroadcastTask{
				Members: members,
				Msg: msg,
			}

			rdb.XAck(context.Background(), "server:" + stream, group, msg.ID)
		}		

		log.Printf("Delivered all %d messages.", len(res[0].Messages))
	}
}

func msg_worker(comms chan domain.BroadcastTask, cp *state.Conn_Pool, worker_id int) {
	for {
		select {
		case new_batch := <-comms:
			var users = make(map[string]bool)
			for _, member := range new_batch.Members {
				val := strings.Split(member, ":")
				users[val[0]] = true
			}
				
			data, _ := json.Marshal(new_batch.Msg.Values)
			for user_id, _ := range users {
				// DONT skip the sender. De-duplicate on client side.
				wc := cp.Get_WS_Conn(user_id)				
				if wc == nil {
					continue
				}
				if len(wc) == cap(wc) {
					log.Print("CHANNEL FULL")
				}
				wc <- data
			}
		}
	}
}

package state

import (
	"sync"

	// "github.com/gorilla/websocket"
)

type Conn_Pool struct {
	// pool       sync.Map
	pool       map[string]chan[]byte
	guild_user map[string]map[string]bool
	user_guild map[string]string
	mu         sync.RWMutex
}

func NewConnPool() *Conn_Pool {
	return &Conn_Pool{
		pool:       make(map[string]chan[]byte),
		guild_user: make(map[string]map[string]bool),
		user_guild: make(map[string]string),
	}
}

func (cn *Conn_Pool) Get_WS_Conn(user_id string) chan[]byte {
	// log.Print("DELIVERING WS FOR USER ID: ", user_id)
	// defer log.Print("DELIVERED WS FOR USER ID: ", user_id)
	cn.mu.RLock()
	defer cn.mu.RUnlock()
	return cn.pool[user_id]
	// ws, ok := cn.pool.Load(user_id)
	// if !ok {
	// 	return nil
	// }
	// return ws.(*websocket.Conn)
}

func (cn *Conn_Pool) Add_Conn(user_id string, wc chan[]byte) {
	// log.Print("ADDING WS FOR USER ID: ", user_id)
	// defer log.Print("ADDED WS FOR USER ID: ", user_id)
	cn.mu.Lock()
	defer cn.mu.Unlock()
	cn.pool[user_id] = wc
	// cn.pool.Store(user_id, ws)
}

func (cn *Conn_Pool) Remove_Conn(user_id string) {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	delete(cn.pool, user_id)
}

// func (cn *Conn_Pool) Change_Guild(user_id string, guild_id string) {
// 	// log.Printf("CHANGE USER ID %s TO GUILD ID %s", user_id, guild_id)
// 	// defer log.Printf("USER ID %s IS NOW ON GUILD ID %s", user_id, guild_id)
// 	prev_guild, ok := cn.user_guild[user_id]
// 	if ok {
// 		delete(cn.guild_user[prev_guild], user_id)
// 	}
// 	cn.user_guild[user_id] = guild_id
// 	if cn.guild_user[guild_id] == nil {
// 		cn.guild_user[guild_id] = make(map[string]bool)
// 	}
// 	cn.guild_user[guild_id][user_id] = true
// }

// func (cn *Conn_Pool) Get_Users_From_Guild(guild_id string) map[string]bool {
// 	return cn.guild_user[guild_id]
// }

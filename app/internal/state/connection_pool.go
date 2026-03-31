package state

import "github.com/gorilla/websocket"

type Conn_Pool struct {
	pool       map[string]*websocket.Conn
	guild_user map[string]map[string]bool
	user_guild map[string]string
}

func NewConnPool() *Conn_Pool {
	return &Conn_Pool{
		pool: make(map[string]*websocket.Conn),
		guild_user: make(map[string]map[string]bool),
		user_guild: make(map[string]string),
	}
}

func (cn *Conn_Pool) Get_WS_Conn(user_id string) *websocket.Conn {
	return cn.pool[user_id]
}

func (cn *Conn_Pool) Add_Conn(user_id string, ws *websocket.Conn) {
	cn.pool[user_id] = ws
}

func (cn *Conn_Pool) Change_Guild(user_id string, guild_id string) {
	prev_guild, ok := cn.user_guild[user_id]
	if ok {
		delete(cn.guild_user[prev_guild], user_id)
	}
	cn.user_guild[user_id] = guild_id
	if cn.guild_user[guild_id] == nil {
		cn.guild_user[guild_id] = make(map[string]bool)
	}
	cn.guild_user[guild_id][user_id] = true
}

func (cn *Conn_Pool) Get_Users_From_Guild(guild_id string) map[string]bool {
	return cn.guild_user[guild_id]
}

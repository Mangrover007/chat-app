<!-- Systems:

I wanna support a few things. First of all:

Servers
    Channels -> two types
    - text channel
    - voice / video channel

Flow of data:
Client sends an HTTP request to https://discord.com/
Request contains info:
    - Server ID
    - Channel ID
    - Payload

MASTER PLAN:
    Move everthing to kubernetes.
        Single Redis deployment working as pubsub broker.
        My application deployment.
 -->

Ruling out plans:
1. Per client dynamic subscriber

Idea was to keep a struct like
```go
type Client struct {
    Conn           *websocket.Conn
    Sub            *redis.PubSub
    Active_Channel string
    // other fields
}
```

and I would have one handler for when the client initiates a
channel change or server change:
```go
func change_handler(w http.ResponseWriter, r *http.Request) {
    server, channel := r.Params() // like /:server/:channel
    chan_name = fmt.Sprintf("/%s/%s", server, channel)
    // singleton idk. only one subscription active at a time
    client.Sub = rdb.Subscribe(Ctx: context.Background(), chan_name)
    w.WriteHeader(200)
}
```

Problem with this: how is client sending message?
Message sent in WS, goes into Queue like this:
```go
func ws_handler(w http.ResponseWriter, r *http.Request) {
    // upgrade connection
    ws := upgrade()
    
    for {
        // read new message arrived
        reader := ws.NextReader()
        data := io.ReadAll(reader)

        // send message to Queue with rdb.Publish
        rdb.Publish(ctx, data, chan_name) //             <--- this is the problem
    }
}
```

What is chan_name? After client switched, chan_name is no longer same.
So I can store that in Client as well, as an Active_Channel field.
That changes the earlier line to this:
```go
rdb.Publish(ctx, data, client.Active_channel)
```

and change handler will now be:
```go
func change_handler(w http.ResponseWriter, r *http.Request) {
    server, channel := r.Params() // like /:server/:channel
    chan_name = fmt.Sprintf("/%s/%s", server, channel)
    // singleton idk. only one subscription active at a time
    client.Sub = rdb.Subscribe(Ctx: context.Background(), chan_name)
    
    // new line
    client.Active_Channel = chan_name
    
    w.WriteHeader(200)
}
```

~~Well ok. Here is another problem:
__Changing subscription of ONE client will change the subscription
of the entire Pod__~~

The problem that was just described was because of a misunderstanding
in my own understanding of Redis pubsub model. It is no more relevant.

Ok so publish problems are taken care of. How do you take care of
listening to the publishes though? Per client, we would need to spin
up a new thread like so:
```go
func ws_handler(w http.ResponseWriter, r *http.Request) {
    // ...
    go sub_rec(client.Sub)
}

func sub_rec(sub *redis.PubSub) {
    for {
        sub.ReceiveMessage()
        // now how do i know which other members to send this msg??
    }
}
```

Which introduces new problem of how we will tell which client to send
the message to in this Pod's client pool. Since every client has an
"Active channel"... maybe I send that in func argument too?

Can't do that because func is only called once.

Send the whole client?
```go
func ws_handler(w http.ResponseWriter, r *http.Request) {
    // ...
    go sub_rec(client)
}

func sub_rec(client *Client) {
    for {
        msg := client.Sub.ReceiveMessage()
        
        for c := range clients {
            if c.Active_Channel == client.Active_Channel {
                // send message
                c.Conn.Write(msg)
            }
        }
    }
}
```

All of this solves mostly all problems with sending and receiving
messages that I know of.

However,
This cannot send notifications, pings (mentions), etc.

Will think about this later.

Glaring issues gemini pointed out with this approach:
A. Redis connection pool is limited. Aroudn 10k connections (pubsub)
and the entire pool is filled.

B. Connection fluctuations from client side, WS reconnects, messages
will not be delivered. The core of this issue is that PubSub model
is fire-and-forget.

C. In sub_rec, for every single subscription, I loop through the
entire list of clients for that Pod. This is way too slow for hundreds
of people chatting simultaneously.

D. Concurrency issues with global read/write to clients[] map.

And hence, this model breaks.

2. Per server connection pool

Idea is to keep a live connection pool of all clients connected to
a server. For example:
```go
var server_pool map[Client]bool // only active users
```

For every server in existence. I guess.
To identify servers, I would need to also manage a map of all servers.
```go
var server_map map[Server]bool
```

and so Server could be
```go
type Server struct {
    conn_pool map[Client]bool // client | is_active
    // other fields if necessary
}

type Client struct {
    WS             *websocket.Conn
    Active_Channel interface{}      // idk what this would be yet
    // other fields if necessary
}
```

I guess conn_pool has all members all the time? I have no idea. We can
come back to this later.

When a connection arrives, add to the server's pool. I imagine to switch
servers or channels, the client would be sending some requests that
contain server_id + channel_id:
```go
func change_handler(w http.ResponseWriter, r *http.Request) {
    server_id, channel_id := r.Params // from params
    prev_server_id := r.Body // from body

    channel := fmt.Sprintf("/%s/%s", server_id, channel_id)
    client.Active_Channel = channel

    // check if the server changed
    // i saw discord keeps a "was" field somewhere so thats how i came
    // up with this
    if server_id != prev_server_id {
        server_map[prev_server_id].conn_pool[client] = false
        server_map[server_id].conn_pool[client] = true
    }
}

func ws_handler(w http.ResponseWriter, r *http.Request) {
    client := Client{
        // fill out all fields
    }

    // since this is a new connection, client is not "active" on any
    // server
    ws := upgrade()

    for {
        // listen for ws writes
        reader := ws.NextReader()
        msg := io.ReadAll(reader)

        // send msg... to what server? to what channel?
        // oh right, active channel
        client.Active_Channel.SendMessage() // ??
    }
}
```

You know. I've been thinking about this. Websockets to send and
receive server messages is just a lot of hassle compared to an
HTTP POST.

I will figure out what to do with HTTP POST and the PubSub and
the message queue models.

Later.

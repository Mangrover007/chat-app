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

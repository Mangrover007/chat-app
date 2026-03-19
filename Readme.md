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

Later is now.

3. HTTP POST request + PubSub / MessageQueue

Either this is trivial or I am missing something. Idea is pretty
simple:

Client sends an HTTP POST request and this is published into a 
Redis Stream (MessageQueue). Then, a worker from sending consumer
group (there may be more than one consumer groups) will send the
message to all members of the server. It's just gonna loop through
it.

Simple example:
```go
type NewMessage struct {
    Content  string
    Username string
    // other fields
}

const consumer_group = rdb.XGroupCreateMkStream(
    context.Background(),
    "msg_broker",
    "worker",
    "$",
    ).Result()

func new_msg_handler(w http.ResponseWriter, r *http.Request) {
    var new_msg NewMessage
    err := json.Unmarshal(r.Body, &new_msg)
    if err != nil {
        w.WriteHeader(400)
    }

    _, err := rdb.XAdd(context.Background(), &redis.XAddArgs{
        Stream: "msg_broker",
        Values: map[string]interface{}{
            // sender | content?
            new_msg.Username: new_msg.Content,
        },
    }).Result()

    if err != nil {
        panic(err)
    }
}
```

Problem: How do I filter servers/channels?

One way is to make a stream for every server/channel.
`msg_broker` needs to be there for every server/channel.

So when a server is created, I can do this...
```go
func create_channel_handler() {
    rdb.XAdd()
    // does not work because Redis does not have a dedicated
    // command just for creating streams.
}
```

Problem: described above.
One way to handle it is to make a separate function that sends
message and makes stream if is not there.

This is more useful. Since every Pod has their own Redis client
running, that means it may happen that a request is routed to a
Pod that does not have the stream already created.

Something like this should work?
```go
func send_message(msg NewMessage, channel string) error {
    res, err := rdb.Exists(channel).Result()
    if err != nil {
        log.Printf("ERROR: %v", err.Error()) // or return err here
    }
    if res == 0 {
        _, err := rdb.XGroupCreateMkStream(
            context.Background(),
            channel,
            fmt.Sprintf("%s%s", channel, "listener"), // group name
            "$",                                      // consume only new messages
            ).Result() // returns string, error

        if err != nil {
            log.Printf("ERROR: %v", err.Error()) // or return err here
        }
    }

    _, err = rdb.XAdd(&redis.XAddArgs{
        Stream: channel,
        Values: map[string]interface{}{
            // sender | content ??
            msg.Username: msg.Content,
        }
    }).Result()

    return err
}

func new_msg_handler(w http.ResponseWriter, r *http.Request) {
    var new_msg NewMessage
    err := json.Unmarshal(r.Body, &new_msg)
    if err != nil {
        w.WriteHeader(400)
    }

    server_id, channel_id := r.Params // from /:serverID/:channelID
    channel := fmt.Sprintf("/%s/%s", server_id, channel_id)
    // because send_message can produce errors i have not handled yet
    defer w.WriteHeader(500)
    err = send_message(msg, channel)
    if err != nil {
        log.Printf("ERROR: %v", err.Error())
    }

    w.WriteHeader(200)
}
```

I will write consumer logic later? Sure.

Later is now.
So the consumer logic is simple. I also would need to spin up a
goroutine. This is still me prototyping.

Because I think I should keep workers in a different Pods...?

```go
func _worker(last_id string, chk_backlog bool = true, stream string, group string) {
    lastid = "0-0"
    for {
        var myid string
        if chk_backlog {
            myid = lastid+
        } else {
            myid = ">"
        }

        items, err := rdb.XReadGroup(&redis.XReadGroupArgs{
            Group: group,
            Consumer: "worker",              // idk generate unique ID or something?
            Streams: []string{stream, myid}, // from docs
            // other fields as necessary
        }).Result()

        if err != nil {
            panic(err)
        }

        // backlog cleared
        if len(items) == 0 {
            chk_backlog = false
            continue
        }

        // process items:
            // get all users of this server
            // send message to them

            // Problem: how do I send message to them?
            // Sharing client pool with other pods is again impractical
            // and now i am stuck AGAIN
    }
}
```

I just realized that it would be impossible (possibly impossible)
to spin up these workers in a separate Pod. The main problem is:
How to order the Pods to start the worker?

Moreover, I imagine redis streams are lived in the same redis DB
in some Pod. I could operate this as a side car?

The issue with a client connection pool is this:

Suppose client connection pool is a map of serverID <--> Client

Suppose a client X is connected Pod A via websocket. The client
pool is local to that Pod, so only Pod A knows client X is
connected to this server.

Now, if client X sends an HTTP POST message, it may be routed to
some other Pod, Pod B. Pod B has his own redis stream and redis
consumer groups and workers. When the message arrives, it IS
published to redis streams and it IS consumed by consumer group
and it IS handed to a worker.

But the worker has no idea about client X. It only knows client Y
is connected to it (because client connection pool is local).

This call for sharing connection pool between Pods. And that, is
impossible.

Solution: a shared service whose sole purpose is to track who is
connected where and how. Taken directly from
[this link](https://ably.com/blog/chat-app-architecture).

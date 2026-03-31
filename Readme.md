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
            channel,                                  // stream name
            fmt.Sprintf("%s%s", channel, "listener"), // group name
            "$",                                      // consume only new messages
            ).Result() // returns string, error

        if err != nil {
            log.Printf("ERROR: %v", err.Error()) // or return err here
        }

        // could / should separate this into its own service
        // i dont know how to sync the redis pods and the worker pods
        // for the same stream tho
        go _worker(
            true, 
            channel,    // stream
            group_name, // group name is fmt.Sprintf("%s%s", channel, "listener")
        )
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

TODO: Learn about `distributed databases` and its design?

```go
func _worker(chk_backlog bool, stream string, group string) {
    var lastid = "0-0"
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

4. Finalizing the design

So I was thinking that making a service for the sole purpose of
tracking connections is similar to the controller-service-model
architecture.

Anyhow, here is how the connection tracking service would be:
```go
type Server struct {
    Conn_pool map[*websocket.Conn]bool
    // other fields
}

var server_pool map[string]Server
var ws_pool     map[*websocket.Conn]bool

func websocket_handler(w http.ResponseWriter, r *http.Request) {
    ws := upgrade()
    ws_pool[ws] = true
}

func change_server_handler(w http.ResponseWriter, r *http.Request) {
    server_id := r.Params
    user_id, server_id_prev := r.Body

    s, ok := server_pool[server_id]
    if !ok {
        server_pool[server_id] = Server{
            Conn_pool: make(map[*websocket.Conn]bool),
            // other fields
        }
    }
    ws := ws_pool[user_id]

    if server_id_prev != nil {
        delete(server_pool[server_id_prev].ws_pool, ws)
    }

    s.Conn_pool[ws] = true
}
```

New problem: How do I listen for messages exactly?

For that, I think I need to rethink how I am doing the message
queue. The entire flow.

- The message arrives as an HTTP POST request to some service.
- This service extracts message data, and pushes it to a queue.
- Then, the connection pool service has an API endpoint to send
messages.
- This API endpoint will assume I give it the necessary data it
needs to fulfill the request.

For example, data like server ID, message, sender, etc.

Between steps 2 and 3, someone needs to hit the API endpoint.
Another service? Where the only thing the service is doing is
listening for any queue activity, and then consuming it.

So a dedicated service for consumer groups? Yeah?

That is for later I guess. For now I can combine the consumer
group and the redis stream into a single Pod. This is already
done in solution 3.

```go
func new_msg_handler() {} // this is the function we need to call through the API
```

And thus, the fan-out to the connection pool service for new
messages rests on the worker function.

Let's see how I can implement it:

```go
var pubsub = redis.Publisher()

func _worker(chk_backlog bool, stream string, group string) {
    var lastid = "0-0"
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

        // fan-out each item (message) to connection pool service
        // pubsub to a middle service for this
        for msg : items {
            pubsub.Publish(msg)   
        }
    }
}
```

And everyone Pod in connection pool service will be listening for
this pubsub.

Alright the entire flow is done. This is the rough diagram I drew
of this architecture:
![please add image]()

5. The previous idea is stupid.

Scaling that is a nightmare because that will make a new topic for
new channels and there are millions and millions of channels. I did
some more reasearch and came across many different articles on how
to scale chat applications.

The one common theme among them was that they made each server
instances subscribed to their own topics. This way, there are as
many topics as there as server instances. This is obviously much
cleaner and scalable.

Here is the rest of the design:

The redis cache is the source of truth for which client is connected
to which internal server instance (Pod). When a new Pod is started,
it starts executing the main function:

```go
func main() {
    // Generate unique ID for this Pod / instance
    server_id UUID
    
    // Unique group ID
    group_id UUID
    err := rdb.XCreateGroupMkStream(
        context.Backgroud(),
        server_id,
        group_id,
        "0",
    ).Err()
    if err != nil {
        log.Print("FATAL ERROR: ", err.Error())
        panic(err)
        return
    }
}
```

In the Redis cache, key value pairs are stored under a keyspace as:
```redis
user:<UUID> <UUID>
```

The key being `user:<UUID>` and the value being `<UUID>` of the Pod,
also called `<server UUID>`.

Note that the server UUID is the internal server's ID. From now on,
the frontend discord "server" will be called groups/guilds.

Each server has a corresponding topic in the Redis store.

The server has a unique ID. It is a consumer of its own topic in the
Redis stream.

Here is the flow of message send and receive:
First, the user is connected to the backend through a websocket. This
means that the user is, essentially, permanently connected to a single
Pod.

Each Pod manages its own internal state, which includes, but is not
limited to:
1. A map of user UUID to websocket Connection
2. A map of user UUID to guild UUID
3. A map of guild UUID to map of user UUID

Map 2 is used so that the Pod can track which user is connected to
which guild. The Pod should also contain an endpoint to change guilds,
to update the mapping so that the Pod can track it correctly.

Map 3 is used to track which guild contains what members. This is
required for quickly sending messages to all active members of a
guild that are connected to one Pod.

The user sends the message through an HTTP POST request on some route
`/:guild_id/:channel_id`. This request comes with a JWT token that stores
the user's UUID and is given to the user when they log in.

So for the purposes of this doc, we will assume that user's request has
the minimum required relevant information for the request to be fulfilled.
Namely, the following fields are considered as minimum requirements:
1. User's UUID
2. User's username

The handler for the specified route extracts the message through the
request body, which contains at least the following fields:
```go
type Payload struct {
    Content string `json:"content"`
}
```

Because we will be getting the guild ID and channel ID through the URI,
we don't need it in the Payload.

For now, the same handler will get all users in the guild (from Postgres)
and iterate through the UUIDs of the users asking Redis store for the
server IDs they are connected to, thus creating a unique set of `server
UUIDs`.

For each server UUID, the same handler will push the message to each
Redis stream `stream:<server UUID>` with at LEAST the following fields:
```py
# payload
{
    'Message':  Content,
    'Username': Username,    # sender, retrieved from token
    'Guild':    GuildID,     # from the URI
    'Channel':  ChannelID,   # from the URI
}
```

The job of the handler is now done, and it will now send a 200 OK response
to the user.

NOTE: I say "for now, the same handler..." because in the future, we might
shift to just pushing the message to a single queue and make a separate
service for handling the processing from the line "For now, the same...".
Or, we can also have multiple queues and rotate in a round-robin style
to further load balance this, but of course, this will introduce more
latency. For now though, a single handler is good enough for the job.

NOTE: storing the user's UUID in the JWT token might be bad practice.
In that case, we would need to make a DB call to get the user's UUID.

Below is a sample implementation of it:
```go
import "pgx/v5"

type Message struct {
    Content string
    Username string // or UUID
    Guild UUID
    Channel UUID
}

type User struct {
    ID UUID
    Username string
    // other fields
}

func msg_handler(w http.ResponseWriter, r *http.Request) {
    p, err := io.ReadAll(r.Body)
    if err != nil {
        log.Print("ERROR: ", err.Error())
        w.WriteHeader(http.StatusInternalServerError)
        return
    }

    var payload Payload
    err = json.Unmarshal(p, &payload)
    if err != nil {
        if errors.Is(err, &json.UnmarshalError) {
            w.WriteHeader(http.StatusBadRequest)
            w.Write([]byte("Bad payload")) // or a better message
            return
        }
        log.Print("ERROR: ", err.Error())
        w.WriteHeader(http.StatusInternalServerError)
        return
    }

    guild_id = r.Header.Get("guild_id")
    channel_id = r.Header.Get("channel_id")

    // From here, either push the message to a queue
    // for it to get consumed by a service that will
    // handle message processing and return.
    // This may be done in the future, but not now.

    // Get all members of the guild
    res, err := psql.Query(
        context.Background(),
        "SELECT user_id FROM user_guild WHERE guild_id = $1",
        guild_id,
    )
    if err != nil {
        log.Print(err.Error())
        w.WriteHeader(http.StatusInternalServerError())
        return
    }

    // Contains the server ID of all members
    // Note that this server ID is internal server ID,
    // not to be confused with guild ID
    servers := make(map[UUID]bool)
    for {
        if !res.Next() {
            break
        }

        var member_id UUID
        err := res.Scan(&member_id)
        if err != nil {
            log.Print(err.Error())
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        // As mentioned, Redis store stores the User ID
        // to internal Server ID for sending message to
        // correct Topics
        serv_id, err := rdb.Get(fmt.Sprintf("user:%s", member_id)).Result()
        if err != nil {
            log.Print(err.Error())
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        servers[serv_id] = true
    }

    // Extract username from token I guess
    // I would need to forward the token to the next
    // service in the future then
    tok = r.Header.Get("token")
    var user User
    _ = jwt.Unmarshal(tok, &user) // or however to unmarshal JWT

    // Not checking for jwt unmarshal error, again,
    // because we are under the assumption that the
    // authentication service has already done that

    var msg = Message{
        Content: payload.Content,
        Username: user.Username,
        GuildID: guild_id,
        ChannelID: channel_id,
    }

    // Each server is a consumer of its own topic
    // (Redis stream key). Message consumption
    // logic is written in consume_msg_handler()
    for _, serv_id := range servers {
        rdb.XAdd(
            context.Background(),
            &redis.XAddArgs{
                Stream: serv_id,
                Values: map[string]interface{}{
                    msg,
                },
            },
        )
    }

    w.WriteHeader(http.StatusOK)
}
```

Now the only problem that remains is how one Pod knows who to send the
message. This is an easy problem to solve.

First, each Pod has a map, as mentioned earlier:
```go
// Map 1 of user UUID to websocket Connection
var conn_pool = make(map[UUID]*websocket.Conn)
```

It is a map of the `<user UUID>` to the websocket connection. The only
entries on this map are users who are directly connected to this Pod.
The handler for ensuring this could be implemented as follows:
```go
func ws_handler (w http.ResponseWriter, r *http.Request) {
    // Assume a separate service running before this has
    // ensured that any request that reaches this stage
    // is authorized with a valid token.

    // The same token is attached to this handler.

    // Note that this service is running separately from
    // the authorization service, specifically AFTER it.
    
    tok = r.Header.Get("token")
    // decode the token into the User struct
    var user User
    Unmarshal(tok, &user)

    ws, err := upgrade(w, r, nil)
    if err != nil {
        log(err.Error())
        w.WriteHeader(http.StatusInternalServerError)
        return
    }

    conn_pool[user.ID] = ws
    // do we need to w.WriteHeader(http.StatusOK) here?
    return 
}
```

Then, for receving messages and sending it over to the user:
```go
// as mentioned earlier, Map 2
var user_guild = make(map[UUID]UUID)

// as mentioned earlier, Map 3
var guild_user = make(map[UUID]map[UUID]bool)

func consume_msg_handler() {
    var chk_backlog = true
    
    for {
        var myid string
        var count = 10
        if chk_backlog {
            myid = "0-0"
        } eles {
            myid = ">"
        }

        res, err := rdb.XReadGroup(
            context.Background(),
            &redis.XReadGroupArgs{
                Streams:  []string(server_id, myid),
                Group:    "group",  // can make this when starting the Pod
                Consumer: "worker", // random ID if want to scale (not now)
                Block:    0,
                Count:    count,
            },
        ).Result()

        if err != nil {
            log.Print("WORKER ERROR: ", err.Error())
            continue
        }

        if len(res) == 0 {
            chk_backlog = false
            continue
        }

        for _, msg := range res.Messages[0] {
            // custom marhsaller/unmarshaller for unpacking msg
            var _msg Message
            err := json.Unmarshal(msg, &_msg)
            if err != nil {
                log.Print(err.Error())
                continue
            }

            guild_id := _msg.GuildID
            members := guild_user[guild_id]
            for _, member := range members {
                w := conn_pool[member].NextWriter()
                fmt.Fprintf(
                    w,
                    "{\"content\":\"%s\",\"username\":\"%s\"}",
                    _msg.Content, _msg.Username,
                )
            }
        }
    }
}
```

Not gonna lie, my brain is not able to understand how discord is
overcoming the insane challenges that they do.

For example, let's say that the service that is directly connected
to the user via Websocket keeps track of the following user
information:
```python
{
    ID: UUID,
    Username: str,
    Guild: UUID,     # active guild
    Channel: UUID,   # active channel
}
```

When the user changes their `guild/channel`, the frontend sends
a piece of information through the Websocket that contains data
for where the guild and channel IDs.

Now, discord message sends are HTTP POST requests that are running
as a separate service. How exactly, does this service know about
the user's information??

It only knows the user's token. Nothing more. Which is also
temporary. The only service that can forward user information to
the next service for handling is if the frontend sends its
content through the Websocket, because that Pod knows this user,
because it is a persistent connection.

That is not how discord operates though and it is the biggest point
of my confusion.

The only way that user information is shared among all services
without the previous service handing it over to the next is if the
information itself is stored in a Redis store cache. In this case,
how would you ensure cleanup? When a user disconnects, how would
we ensure cleanup?

I'm not even thinking about scaling database or cache right now.
That headache is for later!

If I forget about clean up, then what would I even choose for the
key? The user's token is temporary. The user's UUID is unknown.

Well actually, the backend gateway could extract the user's UUID
and attach it to the request for me... Hmm...

Server switching can be updated as well... because the gateway
gives me UUID, so I can just update the user information. On Redis,
it could like something like:

```md
1) user:<UUID>
2) <base64 encoded user information>
```

User information fields already mentioned earlier.

-----------------------------------------------------------------------

Few terms I should know:

1. Back-pressure
2. Pipelining/Batching

--------------------------------------------------------------------------

FLOW:

1. For HTTP POST request to send a message to a guild:
`main.main.go --> main.router.go --> http.router.go --> http.handler.go --> service.message.go`

2. For connecting via WS:
`main.main.go --> main.router.go --> websocket.router.go --> websocket.handler.go --> service.websocket.go`

When a user first connects, need to do the following:
1. Upgrade their connection to WS
2. Register which internal server / Pod the UUID is connected to

Number 2. will insert to Redis store: `user:<user_UUID> <server_UUID>`

Message receives will happen from a `stream.consumer.go` file. It should be
started right after when the server starts up. Because it will shove down
messages down websocket connections, there is a third thing that needs to
happen:
3. Register the websocket connection to the global connection pool, so in
`map[UUID]*websocket.Conn`
    
    chat-app/
    ├── cmd/
    │   └── server/
    │       └── main.go              # entry point (bootstraps everything)
    │
    ├── internal/
    │   ├── app/
    │   │   ├── server.go           # app initialization (wires dependencies)
    │   │   └── lifecycle.go        # startup/shutdown logic
    │   │
    │   ├── config/
    │   │   └── config.go           # env config (redis, postgres, etc.)
    │   │
    │   ├── transport/
    │   │   ├── http/
    │   │   │   ├── router.go       # routes setup
    │   │   │   ├── middleware.go   # auth, logging, etc.
    │   │   │   └── handlers/
    │   │   │       ├── message.go  # POST /:guild/:channel
    │   │   │       └── guild.go    # switch guild endpoint
    │   │   │
    │   │   └── websocket/
    │   │       ├── handler.go      # ws_handler
    │   │       ├── hub.go          # connection pool (conn_pool)
    │   │       └── client.go       # wrapper around websocket conn
    │   │
    │   ├── domain/
    │   │   ├── message.go         # Message struct
    │   │   ├── user.go            # User struct
    │   │   └── guild.go
    │   │
    │   ├── service/
    │   │   ├── message_service.go # core business logic (fanout logic)
    │   │   ├── guild_service.go   # guild membership logic
    │   │   └── connection_service.go # manages maps (guild_user, user_guild)
    │   │
    │   ├── repository/
    │   │   ├── postgres/
    │   │   │   └── guild_repo.go  # fetch guild members
    │   │   │
    │   │   └── redis/
    │   │       ├── user_map.go    # user:<uuid> → server_id
    │   │       └── stream.go      # XADD, XREADGROUP logic
    │   │
    │   ├── stream/
    │   │   ├── consumer.go        # consume_msg_handler
    │   │   └── producer.go        # push to streams
    │   │
    │   ├── auth/
    │   │   └── jwt.go             # JWT parsing (decouple from handler)
    │   │
    │   ├── state/
    │   │   ├── connection_pool.go # conn_pool
    │   │   ├── user_guild.go      # map[user]guild
    │   │   └── guild_user.go      # map[guild]users
    │   │
    │   └── util/
    │       └── logger.go
    │
    ├── pkg/                       # reusable (optional)
    │   └── uuid/
    │       └── uuid.go
    │
    ├── scripts/
    │   └── run.sh
    │
    ├── go.mod
    └── README.md

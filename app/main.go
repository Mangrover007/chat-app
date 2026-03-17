package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"io"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var PS_PORT = "6379"
var APP_PORT = "4200"

const (
	channel_name = "pubsub"
	// how the fuck do you extend this to multiple channels
	// without spinning new pods for each server
	//
	// routing? through routing alone is also impossible
	// because to send the message to a certain URI means
	// to send the message to a service, which mean making
	// new service for each server
	//
	// for example, we have two channels: A and B with IDs
	// <chan-A> and <chan-B>
	//
	// routes:  channels/<chan-A> | channels/<chan-B>
	// handler: channels/:chanID
	// logic:   {
	//   		    > Message arrives on this function
	//              > Function publishes the message
	//              > Who the fuck consumes it now?
	//              > One subscription for each server?
	//              > This way all Pods will be subscriber to
	//                literally every server in the fcking world
	//                and this would be a huge waste of resources
	//                because one subscriber means one thread.
	//          }
	//
	// one thread for each channel?
	// how would i create a new thread on every pod if a new
	// channel is created??
	//
	// global chat handler?
	//
	//
	// Here is the master plan:
	// New Topic for every {<serverID><channelID>}.
	// This means one thread for at least one subscriber.
	// Would need to be scaled if channel is very very very busy?
	// Not really. One million messages at once?
	//
	// > The main problem with this would be:
	// wasted resources on inactive channels (and servers)
	// > Potential solution: kill threads with inactive channels
	// > Then i would need to track which channels are active and which
	// are not
	// > or wait no its one topic not one thread.
)

var rdb *redis.Client
var sub *redis.PubSub

type ChatRequest struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

type Client struct {
	Conn        *websocket.Conn
	Username    string
	WriteAccess bool
}

var clients = make(map[Client]bool)
var _wa = make(map[string]bool)

func chat_handler() {
	for {
		data, err := sub.ReceiveMessage(context.Background())
		if err != nil {
			fmt.Printf("Warning: Failed to get message: %v", err.Error())
			continue
		}

		var msg ChatRequest
		err = json.Unmarshal([]byte(data.Payload), &msg)
		if err != nil {
			fmt.Printf("Could not unmarshal WS payload: %v", err.Error())
			continue
		}

		for ws := range clients {
			if ws.Username != msg.Username {
				writer, err := ws.Conn.NextWriter(1)
				if err != nil {
					fmt.Printf("Failed to get a writer for WS: %v", err.Error())
					continue
				}

				_, err = writer.Write([]byte(data.Payload))
				if err != nil {
					fmt.Printf("Failed to write message to WS: %v", err.Error())
					continue
				}

				writer.Close()
			}
		}
	}
}

func websocket_handler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("token")
	var wa = false
	if err == nil {
		token, _ := jwt.Parse(cookie.Value, func(_ *jwt.Token) (any, error) {
			return []byte(SECRET), nil
		})
		if token.Valid {
			claims := token.Claims.(jwt.MapClaims)
			_wa[claims["email"].(string)] = true
			wa = true
		}
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to connect to WS: %v", err.Error())
		return
	}

	client := Client{
		Username:    "username",
		Conn:        ws,
		WriteAccess: wa,
	}

	defer ws.Close()
	defer delete(clients, client)

	clients[client] = true
	fmt.Printf("%+v\n", client)

	for {
		if !client.WriteAccess {
			continue
		}

		_, reader, err := ws.NextReader()
		if err != nil {
			fmt.Printf("Failed to get a reader for WS: %v", err.Error())
			continue
		}

		p, err := io.ReadAll(reader)
		if err != nil {
			fmt.Printf("Failed to read message from WS: %v", err.Error())
			continue
		}

		var msg string = string(p)
		log.Printf("Message: %s\n", msg)
		rdb.Publish(context.Background(), channel_name, msg)
	}
}

type User struct {
	// other DB fields
	Username string
	Password string
	Email    string
}

type RegisterPayload struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

const HASH_COST = 7
const SECRET = "dont-reveal-this-secret-;)"

// temporary, email --> User
var registered_users = make(map[string]*User)

func register_handler(w http.ResponseWriter, r *http.Request) {
	p, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		log.Printf("Error: %v", err.Error())
		return
	}
	var rp RegisterPayload
	err = json.Unmarshal(p, &rp)
	if err != nil {
		// handle SyntaxError, UnmarshalTypeError, Other
		if errors.Is(err, &json.SyntaxError{}) ||
			errors.Is(err, &json.UnmarshalTypeError{}) {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(500)
		}
		log.Printf("Error: %v", err.Error())
		return
	}

	// store in DB or cache
	password_hash, err := bcrypt.GenerateFromPassword(
		[]byte(rp.Password),
		HASH_COST,
	)
	if err != nil {
		w.WriteHeader(500)
		log.Printf("Error: %v", err.Error())
		return
	}

	var user = User{
		Username: rp.Username,
		Password: string(password_hash),
		Email:    rp.Email,
	}

	registered_users[rp.Email] = &user
	w.WriteHeader(201)
}

func login_handler(w http.ResponseWriter, r *http.Request) {
	p, err := io.ReadAll(r.Body)
	if err != nil {
		// handle errors
		w.WriteHeader(400)
		log.Printf("Error: %v", err.Error())
		return
	}

	var lp LoginPayload
	err = json.Unmarshal(p, &lp)
	if err != nil {
		w.WriteHeader(500)
		log.Printf("Error: %v", err.Error())
		return
	}

	user := registered_users[lp.Email]
	if user == nil {
		w.Write([]byte("Register first"))
		w.WriteHeader(401)
		return
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(lp.Password),
	)
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			w.WriteHeader(401)
		} else {
			w.WriteHeader(500)
		}
		log.Printf("Error: %v", err.Error())
		return
	}

	// json web token
	claims := jwt.MapClaims{
		"username": user.Username,
		"email":    user.Email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(SECRET))
	if err != nil {
		w.WriteHeader(500)
		log.Printf("Error: %v", err.Error())
		return
	}

	cookie := http.Cookie{
		Name:    "token",
		Value:   signed,
		Expires: time.Now().Add(time.Hour * 24),
		// other stuff
	}

	http.SetCookie(w, &cookie)
	_wa[lp.Email] = true
	w.WriteHeader(200)
}

func main() {
	if os.Getenv("PS_PORT") != "" {
		PS_PORT = os.Getenv("PS_PORT")
	}

	if os.Getenv("APP_PORT") != "" {
		APP_PORT = os.Getenv("APP_PORT")
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     "pubsub-service:" + PS_PORT,
		Password: "",
		DB:       0,
		Protocol: 2,
	})

	ctx := context.Background()
	log.Print(rdb.Ping(ctx))

	sub = rdb.Subscribe(ctx, channel_name)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/chat", websocket_handler)
	mux.HandleFunc("/register", register_handler)
	mux.HandleFunc("/login", login_handler)
	server := http.Server{
		Addr:    ":" + APP_PORT,
		Handler: mux,
	}

	// subscriber thread
	go chat_handler()
	// spawning new threads to test if all of them fkn consume the publish
	go chat_handler()
	go chat_handler()
	go chat_handler()

	log.Printf("Server listening on port %s", APP_PORT)
	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("Failed to start server: %v", err.Error())
	}
}

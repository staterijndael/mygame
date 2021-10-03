package endpoint

import (
	"context"
	"encoding/json"
	"log"
	"mygame/internal/models"
	"mygame/tools/jwt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Role int

const (
	User Role = iota
	Leader
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	id uint64

	token string

	role Role

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}

			break
		}

		var event ClientEvent

		err = json.Unmarshal(message, &event)
		if err != nil {
			c.conn.WriteMessage(1, []byte("incorrect data"))

			continue
		}

		c.hub.game.eventChannel <- &event
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func (e *Endpoint) serveWs(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	ctx = context.WithValue(ctx, "JWT_KEY", e.configuration.JWT.SecretKey)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	accessToken := r.Header.Get("Authorization")
	if accessToken == "" {
		conn.WriteMessage(1, []byte("access token is empty"))
		conn.Close()

		return
	}

	token, err := jwt.ParseJWT([]byte(e.configuration.JWT.SecretKey), accessToken)
	if err != nil {
		conn.WriteMessage(1, []byte("token parse error "+err.Error()))
		conn.Close()

		return
	}

	if token.ExpiresAt < time.Now().Unix() {
		conn.WriteMessage(1, []byte("token expired"))
		conn.Close()

		return
	}

	var createGame models.CreateGame
	var joinGame models.JoinGame

	_, msg, err := conn.ReadMessage()

	errCreateGame, errJoinGame := json.Unmarshal(msg, &createGame), json.Unmarshal(msg, &joinGame)
	if errCreateGame != nil && errJoinGame != nil {
		conn.WriteMessage(1, []byte("invalid event"))
		conn.Close()

		return
	}

	var hub *Hub
	var role Role
	if errCreateGame != nil {
		game := &Game{
			Name:   "someRandomName",
			Author: "someRandomAuthor",
			Date:   "25.02.2021",
			Rounds: []*Round{
				{
					Id:   1,
					Name: "firstRound",
					Themes: []*Theme{
						{
							Id:   1,
							Name: "firstTheme",
							Quests: []*Question{
								{
									Id:    1,
									Price: 500,
									Objects: []*Object{
										{
											Id:   1,
											Type: Text,
											Src:  "someText",
										},
										{
											Id:   2,
											Type: Image,
											Src:  "./image.png",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		hub = registerHub(ctx, game)

		hub.opts.Name = createGame.Name
		hub.opts.Password = createGame.Password
		hub.opts.MaxPlayers = createGame.MaxPlayers
		// todo: parsing pack
		role = Leader
	} else {
		foundHub, ok := hubs[joinGame.HubID]
		if !ok {
			conn.WriteMessage(1, []byte("incorrect hub id"))
			conn.Close()

			return
		}

		hub = foundHub
		role = User
	}

	if hub.opts.MaxPlayers == len(hub.clients)-1 {
		conn.WriteMessage(1, []byte("players limit reached"))
		conn.Close()

		return
	}

	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256), token: accessToken, role: role, id: token.ID}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}

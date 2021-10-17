package endpoint

import (
	"context"
	"encoding/json"
	"go.uber.org/zap"
	"log"
	"mygame/internal/models"
	"mygame/internal/singleton"
	"mygame/tools/helpers"
	"mygame/tools/jwt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 120 * time.Second

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

		type userEvent struct {
			Type EventType
			Data json.RawMessage
		}

		var usrEvent userEvent

		err = json.Unmarshal(message, &usrEvent)
		if err != nil {
			c.conn.WriteMessage(1, []byte("incorrect data"))

			continue
		}

		event := &ClientEvent{
			Type:  usrEvent.Type,
			Token: c.token,
			Data:  usrEvent.Data,
		}

		if event.Type == "" {
			c.conn.WriteMessage(1, []byte("incorrect event type"))

			continue
		}

		c.hub.game.eventChannel <- event
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
	//example how to use logger
	ctx := e.CreateContext(w, r)

	// example how to get user request token
	//requestToken := ctx.Value(RequestTokenContext).(string)

	ctx = context.WithValue(ctx, "JWT_KEY", e.configuration.JWT.SecretKey)
	ctx = context.WithValue(ctx, "PACKS_PATH", e.configuration.Pack.Path)

	///example how to use logger
	logger := ctx.Value(LoggerContext).(*zap.Logger)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		//example how to use logger
		logger.Error(
			"websocket connection error",
			zap.Error(err),
		)

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

	type ConnectType struct {
		Type string
		Data json.RawMessage
	}

	var connectType ConnectType

	err = json.Unmarshal(msg, &connectType)
	if err != nil {
		conn.WriteMessage(1, []byte("invalid event"))
		conn.Close()

		return
	}

	var hub *Hub
	var role Role
	if connectType.Type == "create" {
		err = json.Unmarshal(connectType.Data, &createGame)
		if err != nil {
			conn.WriteMessage(1, []byte("invalid event data"))
			conn.Close()

			return
		}

		if createGame.Name == "" {
			conn.WriteMessage(1, []byte("invalid game name"))
			conn.Close()

			return
		} else if createGame.Password == "" {
			conn.WriteMessage(1, []byte("invalid password"))
			conn.Close()

			return
		} else if createGame.MaxPlayers < 1 || createGame.MaxPlayers > 8 {
			conn.WriteMessage(1, []byte("incorrect players count"))
			conn.Close()

			return
		}

		parser := NewParser(ctx.Value("PACKS_PATH").(string))

		err = parser.ParsingSiGamePack(string(createGame.PackUID[:]) + ".zip")
		if err != nil {
			conn.WriteMessage(1, []byte("invalid parsing si game pack"))
			conn.Close()
		}

		err = parser.InitMyGame()
		if err != nil {
			conn.WriteMessage(1, []byte("invalid init si game pack"))
			conn.Close()
		}

		game := parser.GetMyGame()

		err := helpers.Unzip(e.configuration.Pack.Path+"/"+string(createGame.PackUID[:])+".zip", e.configuration.PackTemporary.Path+"/"+string(createGame.PackUID[:]))
		if err != nil {
			conn.WriteMessage(1, []byte("internal error: cannot unzip pack archive"))
			conn.Close()
		}

		singleton.IncTemporaryPack(createGame.PackUID)

		hub = registerHub(ctx, game, e.configuration)

		hub.opts.Name = createGame.Name
		hub.opts.Password = createGame.Password
		hub.opts.MaxPlayers = createGame.MaxPlayers
		// todo: parsing pack
		role = Leader
	} else if connectType.Type == "join" {
		err = json.Unmarshal(connectType.Data, &joinGame)
		if err != nil {
			conn.WriteMessage(1, []byte("invalid event data"))
			conn.Close()

			return
		}

		foundHub, ok := hubs[joinGame.HubID]
		if !ok {
			conn.WriteMessage(1, []byte("incorrect hub id"))
			conn.Close()

			return
		}

		hub = foundHub
		role = User
	} else {
		conn.WriteMessage(1, []byte("incorrect connect type"))
		conn.Close()

		return
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

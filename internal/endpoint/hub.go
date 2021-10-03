package endpoint

import (
	"context"
)

var hubs = make(map[int]*Hub)

type Hub struct {
	// Registered clients.
	clients map[string]*Client

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	close chan struct{}

	opts Options

	game *Game
}

type Options struct {
	Name     string
	Password string

	MaxPlayers int
}

func registerHub(ctx context.Context, game *Game) *Hub {
	hub := newHub(ctx, game)
	go hub.run()

	hubs[len(hubs)+1] = hub

	return hub
}

func newHub(ctx context.Context, game *Game) *Hub {
	hub := &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[string]*Client),
		game:       game,
	}

	game.eventChannel = make(chan *ClientEvent)
	game.players = make(map[*Client]*Player)
	game.playersTokenByQueueID = make(map[int]string)
	game.playersQueueIDByToken = make(map[string]int)

	game.hub = hub

	go game.runGame(ctx)

	return hub
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client.token] = client

			event := ClientEvent{
				Type:  Join,
				Token: client.token,
			}

			h.game.eventChannel <- &event
		case client := <-h.unregister:
			if _, ok := h.clients[client.token]; ok {
				delete(h.clients, client.token)
				close(client.send)
			}

			event := ClientEvent{
				Type:  Disconnect,
				Token: client.token,
			}

			h.game.eventChannel <- &event
		case message := <-h.broadcast:
			for _, client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client.token)
				}
			}
		case <-h.close:
			for _, client := range h.clients {
				h.unregister <- client
			}

			break
		}
	}
}

package models

type CreateGame struct {
	Name       string
	Password   string
	MaxPlayers int
	PackUID    string
}

type JoinGame struct {
	HubID int
}

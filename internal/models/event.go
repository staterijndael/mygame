package models

type CreateGame struct {
	Name       string   `json:"name"`
	Password   string   `json:"password"`
	MaxPlayers int      `json:"max_players"`
	PackUID    [32]byte `json:"pack_uid"`
}

type JoinGame struct {
	HubID int `json:"hub_id"`
}

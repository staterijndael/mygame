package endpoint

import (
	"context"
	"encoding/json"
	"log"
	"mygame/tools/jwt"
	"time"
)

type EventType string

const (
	Join          EventType = "join"
	Disconnect    EventType = "disconnect"
	GetQuest      EventType = "get_quest"
	ChooseQuest   EventType = "choose_quest"
	GiveAnswer    EventType = "give_answer"
	DeclineAnswer EventType = "decline_answer"
	AcceptAnswer  EventType = "accept_answer"
)

var roleByEvent = map[EventType][]Role{
	Join:          {User, Leader},
	Disconnect:    {User, Leader},
	GetQuest:      {User},
	GiveAnswer:    {User},
	DeclineAnswer: {Leader},
	AcceptAnswer:  {Leader},
	ChooseQuest:   {User},
}

type ServerEventType string

const (
	GreetingsServer      ServerEventType = "greetings_server"
	ReadingRoundServer   ServerEventType = "reading_round"
	ReadingThemesServer  ServerEventType = "reading_themes_server"
	WallServer           ServerEventType = "wall_server"
	GetQuestServer       ServerEventType = "get_quest_server"
	JoinServer           ServerEventType = "join_server"
	DisconnectServer     ServerEventType = "disconnect_server"
	ChooseQuestServer    ServerEventType = "choose_quest_server"
	TakenQuestServer     ServerEventType = "taken_quest_server"
	ScoreChangedServer   ServerEventType = "score_changed"
	AnswerAcceptedServer ServerEventType = "answer_accepted_server"
	AnswerDeclinedServer ServerEventType = "answer_declined_server"
	FinalServer          ServerEventType = "final_server"
)

type ClientEvent struct {
	Type  EventType
	Token string
	Data  json.RawMessage
}

type ChooseQuestClientEvent struct {
	ThemeID    int
	QuestionID int
}

type Step int

const (
	WaitingStart Step = iota
	Grettings
	ReadingRound
	ReadingThemes
	ChooseQuestion
	Getting
	Answering
	Pause
	Final
)

type ServerEvent struct {
	Type ServerEventType
	Exp  int64
	Data interface{}
}

type JoinServerEvent struct {
	UserID   uint64
	Nickname string
	ImageUID string
}

type DisconnectServerEvent struct {
	UserID uint64
}

type GreetingsServerEvent struct {
	Name   string
	Author string
	Date   string
}

type ReadingRoundServerEvent struct {
	Name string
}

type ReadingThemesServerEvent struct {
	ThemeNames []string
}

type WallServerEvent struct {
	Themes []*Theme
}

type ChooseQuestServerEvent struct {
	ThemeID    int
	QuestionID int
}

type TakenQuestServerEvent struct {
	UserID uint64
}

type GetQuestServerEvent struct {
	UserID uint64
}

type ScoreChangedServerEvent struct {
	UserID uint64
	Score  int
}

type FinalServerEvent struct {
	WinnerID uint64
}

type Game struct {
	Name   string
	Author string
	Date   string
	Rounds []*Round

	hub                    *Hub
	players                map[*Client]*Player
	playersQueueIDByUserID map[uint64]int
	playersUserIDByQueueID map[int]uint64

	eventChannel chan *ClientEvent

	currentStep     Step
	currentPlayerID uint64

	currentRound    int
	currentTheme    int
	currentQuestion int
}

type Player struct {
	client *Client
	score  int
}

type Round struct {
	Id     int
	Name   string
	Themes []*Theme
}

type Theme struct {
	Id     int
	Name   string
	Quests []*Question
}

type ObjectType int

const (
	Text ObjectType = iota
	Image
	MP3
	Video
)

type Question struct {
	Id      int
	Price   int
	Objects []*Object
}

type Object struct {
	Id   int
	Type ObjectType
	Src  string
}

func (game *Game) runGame(ctx context.Context) {
	jwtKey := ctx.Value("JWT_KEY").(string)

	game.currentStep = WaitingStart
	ticker := time.NewTicker(5 * time.Minute)

	defer ticker.Stop()

	for {
		select {
		case event := <-game.eventChannel:
			token, err := jwt.ParseJWT([]byte(jwtKey), event.Token)
			if err != nil {
				var client *Client
				for _, cl := range game.hub.clients {
					if cl.token == event.Token {
						client = cl
					}
				}

				if client != nil {
					client.conn.WriteMessage(1, []byte("token parse error "+err.Error()))
					client.conn.Close()

					game.hub.unregister <- client
				}

				return
			}

			if token.ExpiresAt < time.Now().Unix() {
				var client *Client
				for _, cl := range game.hub.clients {
					if cl.token == event.Token {
						client = cl
					}
				}

				if client != nil {
					client.conn.WriteMessage(1, []byte("token expired "+err.Error()))
					client.conn.Close()

					game.hub.unregister <- client
				}

				return
			}

			accessedRoles := roleByEvent[event.Type]
			var accessed bool
			for _, role := range accessedRoles {
				if role == game.hub.clients[token.ID].role {
					accessed = true
				}
			}

			if !accessed {
				game.hub.clients[token.ID].conn.WriteMessage(1, []byte("permission denied"))

				continue
			}

			var newDuration time.Duration

			switch event.Type {
			case Join:
				var firstPlayer *Player
				for _, pl := range game.players {
					firstPlayer = pl
					break
				}

				if len(game.players) == 1 && firstPlayer.client.role == Leader {
					game.currentPlayerID = token.ID
					game.playersQueueIDByUserID[token.ID] = len(game.playersQueueIDByUserID) + 1
					game.playersUserIDByQueueID[len(game.playersUserIDByQueueID)+1] = token.ID
				}

				game.players[game.hub.clients[token.ID]] = &Player{
					client: game.hub.clients[token.ID],
					score:  0,
				}

				// todo: getting user image
				joinServer := JoinServerEvent{
					UserID:   token.ID,
					Nickname: token.Login,
				}

				game.sendServerEvent(JoinServer, joinServer, 0)
			case Disconnect:
				if _, ok := game.players[game.hub.clients[token.ID]]; ok {
					delete(game.players, game.hub.clients[token.ID])
				}

				disconnectServer := DisconnectServerEvent{
					UserID: token.ID,
				}

				game.sendServerEvent(DisconnectServer, disconnectServer, 0)
			case ChooseQuest:
				var clientEvent ChooseQuestClientEvent

				err = json.Unmarshal(event.Data, &clientEvent)
				if err != nil {
					log.Println(err)
					continue
				}

				game.currentTheme = clientEvent.ThemeID
				game.currentQuestion = clientEvent.QuestionID

				game.currentStep = Getting
				newDuration = 10 * time.Second

				chooseQuest := ChooseQuestServerEvent{
					ThemeID:    clientEvent.ThemeID,
					QuestionID: clientEvent.QuestionID,
				}

				game.sendServerEvent(ChooseQuestServer, chooseQuest, time.Now().In(time.UTC).Add(newDuration).Unix())
			case GetQuest:
				if game.currentStep == Getting {
					player := game.players[game.hub.clients[token.ID]]
					if player.client.role == User {
						game.currentStep = Answering
						game.currentPlayerID = game.players[game.hub.clients[token.ID]].client.id

						newDuration = 20 * time.Second

						takenQuest := TakenQuestServerEvent{
							UserID: token.ID,
						}

						game.sendServerEvent(TakenQuestServer, takenQuest, time.Now().In(time.UTC).Add(newDuration).Unix())
					}
				}
			case AcceptAnswer:
				var found bool
				for _, theme := range game.Rounds[game.currentRound-1].Themes {
					for _, question := range theme.Quests {
						if question.Price >= 0 && question.Id != game.currentQuestion {
							found = true
						}
					}
				}
				if !found {
					if len(game.Rounds) > game.currentRound-1 {
						game.currentRound++
						game.currentStep = ChooseQuestion

						newDuration = 10 * time.Second
					} else {
						game.currentStep = Final
						newDuration = 5 * time.Minute

						var winnerID uint64
						var maxScore int
						for _, player := range game.players {
							if player.score > maxScore {
								maxScore = player.score
								winnerID = player.client.id
							}
						}

						game.sendServerEvent(FinalServer, FinalServerEvent{WinnerID: winnerID}, time.Now().In(time.UTC).Add(newDuration).Unix())
					}
				} else {
					game.currentStep = ChooseQuestion
					newDuration = 10 * time.Second
				}

				curQuest := game.Rounds[game.currentRound-1].Themes[game.currentTheme-1].Quests[game.currentQuestion-1]
				game.players[game.hub.clients[game.currentPlayerID]].score += curQuest.Price

				if uint64(len(game.players)) > game.currentPlayerID {
					game.currentPlayerID = game.playersUserIDByQueueID[game.playersQueueIDByUserID[game.currentPlayerID]+1]
				} else {
					game.currentPlayerID = game.playersUserIDByQueueID[1]
				}

				scoreChanged := ScoreChangedServerEvent{
					UserID: game.currentPlayerID,
					Score:  game.players[game.hub.clients[game.currentPlayerID]].score,
				}

				game.sendServerEvent(AnswerAcceptedServer, nil, time.Now().In(time.UTC).Add(newDuration).Unix())
				game.sendServerEvent(ScoreChangedServer, scoreChanged, 0)

			case DeclineAnswer:
				var found bool
				for _, theme := range game.Rounds[game.currentRound-1].Themes {
					for _, question := range theme.Quests {
						if question.Price >= 0 && question.Id != game.currentQuestion {
							found = true
						}
					}
				}
				if !found {
					if len(game.Rounds) > game.currentRound-1 {
						game.currentRound++
						game.currentStep = ChooseQuestion

						newDuration = 10 * time.Second
					} else {
						game.currentStep = Final
						newDuration = 5 * time.Minute

						var winnerID uint64
						var maxScore int
						for _, player := range game.players {
							if player.score > maxScore {
								maxScore = player.score
								winnerID = player.client.id
							}
						}

						game.sendServerEvent(AnswerDeclinedServer, nil, time.Now().In(time.UTC).Add(newDuration).Unix())
						game.sendServerEvent(FinalServer, FinalServerEvent{WinnerID: winnerID}, 0)
					}
				} else {
					game.currentStep = ChooseQuestion
					newDuration = 10 * time.Second
				}

				curQuest := game.Rounds[game.currentRound-1].Themes[game.currentTheme-1].Quests[game.currentQuestion-1]
				game.players[game.hub.clients[game.currentPlayerID]].score -= curQuest.Price

				if uint64(len(game.players)) > game.currentPlayerID {
					game.currentPlayerID = game.playersUserIDByQueueID[game.playersQueueIDByUserID[game.currentPlayerID]+1]
				} else {
					game.currentPlayerID = game.playersUserIDByQueueID[1]
				}

				scoreChanged := ScoreChangedServerEvent{
					UserID: game.currentPlayerID,
					Score:  game.players[game.hub.clients[game.currentPlayerID]].score,
				}

				game.sendServerEvent(ScoreChangedServer, scoreChanged, time.Now().In(time.UTC).Add(newDuration).Unix())
			}

			if newDuration != 0 {
				ticker.Stop()
				ticker = time.NewTicker(newDuration)
			}
		case <-ticker.C:
			var newDuration time.Duration

			switch game.currentStep {
			case WaitingStart:
				game.hub.close <- struct{}{}
			case Grettings:
				if len(game.Rounds) > game.currentRound-1 {
					game.currentStep = ReadingRound
					game.currentRound++

					newDuration = 4 * time.Second
				} else {
					game.currentStep = Final

					newDuration = 5 * time.Minute
				}

				round := game.Rounds[game.currentRound]

				readingRound := ReadingRoundServerEvent{
					Name: round.Name,
				}

				game.sendServerEvent(ReadingRoundServer, readingRound, time.Now().In(time.UTC).Add(newDuration).Unix())
			case ReadingRound:
				game.currentStep = ReadingThemes

				round := game.Rounds[game.currentRound]

				newDuration = time.Duration(len(round.Themes)) * 3 * time.Second

				themeNames := make([]string, 0, len(round.Themes))
				for _, theme := range round.Themes {
					themeNames = append(themeNames, theme.Name)
				}

				readingThemes := ReadingThemesServerEvent{
					ThemeNames: themeNames,
				}

				game.sendServerEvent(ReadingThemesServer, readingThemes, time.Now().In(time.UTC).Add(newDuration).Unix())
			case ReadingThemes:
				game.currentStep = ChooseQuestion

				newDuration = 10 * time.Second

				round := game.Rounds[game.currentRound]

				wall := WallServerEvent{
					Themes: round.Themes,
				}

				game.sendServerEvent(WallServer, wall, time.Now().In(time.UTC).Add(newDuration).Unix())
			case ChooseQuestion:
				game.currentStep = Getting

				round := game.Rounds[game.currentRound]
				var themeID int
				var quest *Question
				for _, theme := range round.Themes {
					for _, question := range theme.Quests {
						if question.Price >= 0 {
							themeID = theme.Id
							quest = question
						}
					}
				}

				game.currentQuestion = quest.Id
				game.currentTheme = themeID

				getQuest := GetQuestServerEvent{
					UserID: game.currentPlayerID,
				}

				newDuration = 10 * time.Second

				game.sendServerEvent(GetQuestServer, getQuest, time.Now().In(time.UTC).Add(newDuration).Unix())

				// todo: send correct answer to leader
			case Getting:
				game.currentStep = ChooseQuestion
				newDuration = 10 * time.Second

				currentQuest := game.Rounds[game.currentRound-1].Themes[game.currentTheme-1].Quests[game.currentQuestion-1]

				currentQuest.Price = -1
			case Answering:
				var found bool
				for _, theme := range game.Rounds[game.currentRound-1].Themes {
					for _, question := range theme.Quests {
						if question.Price >= 0 && question.Id != game.currentQuestion {
							found = true
						}
					}
				}
				if !found {
					if len(game.Rounds) > game.currentRound-1 {
						game.currentRound++
						game.currentStep = ChooseQuestion

						newDuration = 10 * time.Second
					} else {
						game.currentStep = Final
						newDuration = 5 * time.Minute

						var winnerID uint64
						var maxScore int
						for _, player := range game.players {
							if player.score > maxScore {
								maxScore = player.score
								winnerID = player.client.id
							}
						}

						game.sendServerEvent(FinalServer, FinalServerEvent{WinnerID: winnerID}, time.Now().In(time.UTC).Add(newDuration).Unix())
					}
				} else {
					game.currentStep = ChooseQuestion
					newDuration = 10 * time.Second
				}

				curQuest := game.Rounds[game.currentRound-1].Themes[game.currentTheme-1].Quests[game.currentQuestion-1]
				game.players[game.hub.clients[game.currentPlayerID]].score -= curQuest.Price

				if uint64(len(game.players)) > game.currentPlayerID {
					game.currentPlayerID = game.playersUserIDByQueueID[game.playersQueueIDByUserID[game.currentPlayerID]+1]
				} else {
					game.currentPlayerID = game.playersUserIDByQueueID[1]
				}

				scoreChanged := ScoreChangedServerEvent{
					UserID: game.currentPlayerID,
					Score:  game.players[game.hub.clients[game.currentPlayerID]].score,
				}

				game.sendServerEvent(ScoreChangedServer, scoreChanged, time.Now().In(time.UTC).Add(newDuration).Unix())
			}

			if newDuration != 0 {
				ticker.Stop()
				ticker = time.NewTicker(newDuration)
			}
		}
	}
}

func (game *Game) sendServerEvent(eventType ServerEventType, event interface{}, exp int64) error {
	serverEvent := ServerEvent{
		Type: eventType,
		Exp:  exp,
		Data: event,
	}

	msg, err := json.Marshal(&serverEvent)
	if err != nil {
		return err
	}

	game.hub.broadcast <- msg

	return nil
}

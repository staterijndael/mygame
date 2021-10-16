package endpoint

import (
	"encoding/xml"
	"io/ioutil"
	"mygame/internal/models"
	"os"
	"strconv"
	"strings"
)

const (
	defaultContentName = "content.xml"

	defaultAudioPath = "/Audio"

	defaultImagesPath = "/Images"

	defaultVideoPath = "/Video"
)

type Parser struct {
	IParser
	hash      [32]byte
	packsPath string
	myGame    *Game
	siGame    *models.Package
}

type IParser interface {
	ParsingSiGamePack(packName string) error
	GetMyGame() *Game
	GetSiGame() *models.Package
	InitMyGame() error
}

func NewParser(packsPath string) IParser {
	return &Parser{
		packsPath: packsPath,
		myGame:    new(Game),
		siGame:    new(models.Package),
	}
}

func (p *Parser) ParsingSiGamePack(packName string) error {
	packContent, err := os.Open(p.packsPath + "/" + packName + "/" + defaultContentName)
	if err != nil {
		return err
	}

	bytePackContent, err := ioutil.ReadAll(packContent)
	if err != nil {
		return err
	}

	err = xml.Unmarshal(bytePackContent, &p.siGame)
	if err != nil {
		return err
	}

	defer packContent.Close()

	return nil
}

func (p *Parser) InitMyGame() error {
	p.myGame = &Game{
		Name:   p.siGame.Name,
		Author: p.siGame.Info.Authors.Author,
		Date:   p.siGame.Date,
	}

	for i, round := range p.siGame.Rounds.Round {
		var themes []*Theme

		for j, theme := range round.Themes.Theme {
			var quests []*Question

			for k, question := range theme.Questions.Question {
				var answer []*Object

				answer = append(answer, &Object{
					Id:   1,
					Type: Answer,
					Src:  strings.Join(question.Right.Answer, " "),
				})

				var scene []*Object

				for z, atom := range question.Scenario.Atom {
					if atom.Type == "" {
						atom.Type = Text.String()
					} else if atom.Type == Video.String() || atom.Type == Image.String() ||
						atom.Type == Audio.String() {
						atom.Text = strings.ReplaceAll(atom.Text, "@", "")
					}

					scene = append(scene, &Object{
						Id:   z + 1,
						Src:  atom.Text,
						Type: ObjectType(atom.Type),
					})
				}

				price, err := strconv.Atoi(question.Price)
				if err != nil {
					return err
				}

				quests = append(quests, &Question{
					Id:     k + 1,
					Price:  price,
					Scene:  scene,
					Answer: answer,
				})
			}

			themes = append(themes, &Theme{
				Id:     j + 1,
				Name:   theme.Name,
				Quests: quests,
			})
		}

		p.myGame.Rounds = append(p.myGame.Rounds, &Round{
			Id:     i + 1,
			Name:   round.Name,
			Themes: themes,
		})
	}

	return nil
}

func (p *Parser) GetMyGame() *Game {
	return p.myGame
}

func (p *Parser) GetSiGame() *models.Package {
	return p.siGame
}

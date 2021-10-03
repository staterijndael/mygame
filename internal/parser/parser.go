package parser

import (
	"encoding/xml"
	"io/ioutil"
	"mygame/config"
	"mygame/internal/endpoint"
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
	config *config.Config
	myGame *endpoint.Game
	siGame *models.Package
}

type IParser interface {
	ParsingSiGamePack(packName string) error
	GetMyGame() *endpoint.Game
	GetSiGame() *models.Package
	InitMyGame() error
}

func NewParser(config *config.Config) IParser {
	return &Parser{
		config: config,
		myGame: new(endpoint.Game),
		siGame: new(models.Package),
	}
}

func (p *Parser) ParsingSiGamePack(packName string) error {
	packContent, err := os.Open(p.config.Pack.Path + "/" + packName + "/" + defaultContentName)
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
	p.myGame = &endpoint.Game{
		Name:   p.siGame.Name,
		Author: p.siGame.Info.Authors.Author,
		Date:   p.siGame.Date,
	}

	for i, round := range p.siGame.Rounds.Round {
		var themes []*endpoint.Theme

		for j, theme := range round.Themes.Theme {
			var quests []*endpoint.Question

			for k, question := range theme.Questions.Question {
				var answer []*endpoint.Object

				answer = append(answer, &endpoint.Object{
					Id:   1,
					Type: endpoint.Answer,
					Src:  strings.Join(question.Right.Answer, " "),
				})

				var scene []*endpoint.Object

				for z, atom := range question.Scenario.Atom {
					scene = append(scene, &endpoint.Object{
						Id:   z + 1,
						Src:  atom.Text,
						Type: endpoint.ObjectType(atom.Type),
					})
				}

				price, err := strconv.Atoi(question.Price)
				if err != nil {
					return err
				}

				quests = append(quests, &endpoint.Question{
					Id:     k + 1,
					Price:  price,
					Scene:  scene,
					Answer: answer,
				})
			}

			themes = append(themes, &endpoint.Theme{
				Id:     j + 1,
				Name:   theme.Name,
				Quests: quests,
			})
		}

		p.myGame.Rounds = append(p.myGame.Rounds, &endpoint.Round{
			Id:     i + 1,
			Name:   round.Name,
			Themes: themes,
		})
	}

	return nil
}

func (p *Parser) GetMyGame() *endpoint.Game {
	return p.myGame
}

func (p *Parser) GetSiGame() *models.Package {
	return p.siGame
}

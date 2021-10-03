package main

import (
	"flag"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"mygame/config"
	"mygame/internal/parser"
	"path/filepath"
)

const defaultPacksPath = "./packs"

var packsPath string

func init() {
	flag.StringVar(&packsPath, "packs-path", defaultPacksPath, "packs path")
	flag.StringVar(&packsPath, "p", defaultPacksPath, "packs path")
}

func main() {
	flag.Parse()
	config, err := parseCfg("./config/config.yaml")
	if err != nil {
		panic(err)
	}

	config.Pack.Path = packsPath

	parser := parser.NewParser(config)

	err = parser.ParsingSiGamePack("Samokhodnaya_pizda")
	if err != nil {
		log.Fatal(err)
	}
	parser.InitMyGame()
	game := parser.GetMyGame()
	log.Println(game)
	//
	//

	//
	//connectionAddr := &database.Connection{
	//	Host:     config.DB.Host,
	//	Port:     config.DB.Port,
	//	User:     config.DB.User,
	//	Password: config.DB.Password,
	//	DBName:   config.DB.DBName,
	//	SSLMode:  config.DB.SSLMode,
	//}
	//
	//connectionAddrStr := database.GenerateAddr(connectionAddr)
	//
	//db, err := database.NewDB(connectionAddrStr)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//endpoint := endpoint.NewEndpoint(db, config)
	//endpoint.InitRoutes()
	//
	//log.Fatal(http.ListenAndServe(":" + strconv.Itoa(config.App.Port), nil))
}

func parseCfg(path string) (*config.Config, error) {
	filename, _ := filepath.Abs(path)
	yamlFile, err := ioutil.ReadFile(filename)

	if err != nil {
		return nil, err
	}

	var config config.Config

	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

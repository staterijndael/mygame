package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"mygame/config"
	"net/http"
	"path/filepath"
	"strconv"
)

func main() {
	config, err := parseCfg("./config/config.yaml")
	if err != nil {
		panic(err)
	}

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(config.App.Port), nil))
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

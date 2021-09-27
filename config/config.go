package config

type Config struct{
	App `yaml:"app"`
}

type App struct {
	Port int `yaml:"port"`
}

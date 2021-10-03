package config

import "time"

type Config struct {
	App `yaml:"app"`
	DB  `yaml:"db"`
	JWT `yaml:"jwt"`
}

type App struct {
	Port int `yaml:"port"`
}

type DB struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"db_name"`
	SSLMode  string `yaml:"ssl_mode"`
}

type JWT struct {
	SecretKey      string        `yaml:"secret_key"`
	ExpirationTime time.Duration `yaml:"expiration_time"`
}

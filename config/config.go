package config

import "time"

type Config struct {
	App  App `yaml:"app"`
	DB   DB  `yaml:"db"`
	JWT  JWT `yaml:"jwt"`
	Pack Pack
}

type App struct {
	Port     int    `yaml:"port"`
	LogLevel string `yaml:"log_level"`
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

type Pack struct {
	Path string
}

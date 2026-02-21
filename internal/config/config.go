package config

import (
	"time"

	"github.com/urfave/cli/v3"
)

type Config struct {
	App
	PostgreSQL
	HTTP
}

type App struct {
	WatchDirectory        string
	ReportsDirectory      string
	DirectoryScanInterval time.Duration
}

type PostgreSQL struct {
	Host     string
	Port     string
	Username string
	Password string
	DBName   string
}

type HTTP struct {
	Host         string
	Port         string
	IdleTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func Load(cmd *cli.Command) *Config {
	return &Config{
		App: App{
			WatchDirectory:        cmd.String("watch-dir"),
			ReportsDirectory:      cmd.String("reports-dir"),
			DirectoryScanInterval: cmd.Duration("scan-interval"),
		},
		PostgreSQL: PostgreSQL{
			Host:     cmd.String("pg-host"),
			Port:     cmd.String("pg-port"),
			Username: cmd.String("pg-username"),
			Password: cmd.String("pg-password"),
			DBName:   cmd.String("pg-dbname"),
		},
		HTTP: HTTP{
			Host:         cmd.String("http-host"),
			Port:         cmd.String("http-port"),
			IdleTimeout:  cmd.Duration("http-idle-timeout"),
			ReadTimeout:  cmd.Duration("http-read-timeout"),
			WriteTimeout: cmd.Duration("http-write-timeout"),
		},
	}
}

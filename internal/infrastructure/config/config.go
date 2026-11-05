package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Env string `env:"ENV" envDefault:"dev"`

	Log      Log
	Database Database

	SSHAddr         string        `env:"SSH_ADDR" envDefault:":2222"`
	SSHHostKeyPath  string        `env:"SSH_HOST_KEY_PATH"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`

	RedisAddr string `env:"REDIS_ADDR" envDefault:"localhost:6379"`

	ChatMaxMembers  int `env:"CHAT_MAX_MEMBERS" envDefault:"4"`
	MessageMaxRunes int `env:"MESSAGE_MAX_RUNES" envDefault:"300"`
	NickMaxLen      int `env:"NICK_MAX_LEN" envDefault:"18"`
}

type Log struct {
	Level  string `env:"LOG_LEVEL"  envDefault:"info"`
	Format string `env:"LOG_FORMAT" envDefault:"text"`
}

type Database struct {
	PgDsn string `env:"PG_DSN,required"`
}

func (c Config) Logger() Log {
	return Log{Level: c.Log.Level, Format: c.Log.Format}
}

func Load[T any]() (T, error) {
	return env.ParseAs[T]()
}

func MustLoad[T any]() T {
	cfg, err := Load[T]()
	if err != nil {
		panic(err)
	}

	return cfg
}

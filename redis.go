package serverconfig

import (
	"fmt"
	"time"
)

// RedisConfig is used for creating a Redis connection or Redis pool.  The MaxIdle, MaxActive, and IdleTimeout
// are applicable for pools only.
type RedisConfig struct {
	Server        string        `yaml:"server" env:"REDISSERVER"`
	User          string        `yaml:"user" env:"REDISUSER"`
	Password      string        `yaml:"password" env:"REDISPASS"`
	DatabaseIndex string        `yaml:"databaseindex"`
	MaxIdle       int           `yaml:"maxidle"`
	MaxActive     int           `yaml:"maxactive"`
	IdleTimeout   time.Duration `yaml:"idletimeout"`
}

func (cfg *RedisConfig) Verify() error {
	if len(cfg.Server) == 0 {
		return fmt.Errorf("missing Redis Server (or REDISSERVER environment variable)")
	}
	if cfg.MaxIdle == 0 {
		cfg.MaxIdle = 3
	}
	if cfg.MaxActive == 0 {
		cfg.MaxActive = 32
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 60 * time.Second
	}
	return nil
}

package config

import "time"

type (
	Config struct {
		App      App
		Cache    Cache
		Database Database
		HTTP     HTTP
		Logger   Logger
		Env      string
	}

	App struct {
		Port    int
		Name    string
		Version string
	}

	Cache struct {
		Addr        string
		Password    string
		PoolSize    int
		MinIdleCons int
		PoolTimeout time.Duration
	}

	Database struct {
		DSN            string
		PoolMax        int32
		ConnAttempts   int
		BaseRetryDelay time.Duration
		MaxRetryDelay  time.Duration
	}

	HTTP struct {
		Host              string
		Post              string
		ReadTimeout       time.Duration
		WriteTimeout      time.Duration
		IdleTimeout       time.Duration
		ShutdownTimeout   time.Duration
		ReadHeaderTimeout time.Duration
	}
	Logger struct {
		Level      string
		Filename   string
		MaxSize    int
		MaxBackups int
		MaxAge     int
	}
)

package config

import "time"

type (
	Config struct {
		App      App      `env-prefix:"APP_"`
		Service  Service  `env-prefix:"SERVICE_"`
		Database Database `env-prefix:"DB_"`
		Cache    Cache    `env-prefix:"CACHE_"`
		HTTP     HTTP     `env-prefix:"HTTP_"`
		Logger   Logger   `env-prefix:"LOGGER_"`
		Env      string   `                      env:"ENV" env-default:"local" validate:"oneof=local dev staging prod"`
	}

	App struct {
		Name    string `env:"NAME"    env-default:"lshortener" validate:"required"`
		Version string `env:"VERSION" env-default:"1.0.0"      validate:"required"`
	}

	Service struct {
		ShortCodeLength int           `env:"SHORT_CODE_LENGTH" env-default:"6"                     validate:"min=4,max=12"`
		DefaultTTL      time.Duration `env:"DEFAULT_TTL"       env-default:"0"`
		BaseURL         string        `env:"BASE_URL"          env-default:"http://localhost:8080" validate:"required,url"`
		MaxRetries      int           `env:"MAX_RETRIES"       env-default:"5"                     validate:"min=1,max=10"`
		QueryLimit      uint64        `env:"QUERY_LIMIT"       env-default:"10"                    validate:"min=1,max=100"`
		RetryDelay      time.Duration `env:"RETRY_DELAY"       env-default:"5m"                    validate:"gte=1ms,lte=1m"`
	}

	Database struct {
		DSN            string        `env:"DSN"              env-default:"postgres://postgres:postgres@localhost:5432/url_shortener?sslmode=disable" validate:"required"`
		PoolMax        int32         `env:"POOL_MAX"         env-default:"20"                                                                        validate:"min=1,max=100"`
		ConnAttempts   int           `env:"CONN_ATTEMPTS"    env-default:"5"                                                                         validate:"min=1,max=10"`
		BaseRetryDelay time.Duration `env:"BASE_RETRY_DELAY" env-default:"100ms"                                                                     validate:"gte=10ms,lte=10s"`
		MaxRetryDelay  time.Duration `env:"MAX_RETRY_DELAY"  env-default:"5s"                                                                        validate:"gte=100ms,lte=30s,gtefield=BaseRetryDelay"`
	}

	Cache struct {
		Addr         string        `env:"ADDR"          env-default:"localhost:6379" validate:"required"`
		Password     string        `env:"PASSWORD"      env-default:""`
		DB           int           `env:"DB"            env-default:"0"              validate:"min=0,max=15"`
		DialTimeout  time.Duration `env:"DIAL_TIMEOUT"  env-default:"5s"             validate:"gte=1s,lte=30s"`
		ReadTimeout  time.Duration `env:"READ_TIMEOUT"  env-default:"3s"             validate:"gte=1s,lte=30s"`
		WriteTimeout time.Duration `env:"WRITE_TIMEOUT" env-default:"3s"             validate:"gte=1s,lte=30s"`
		PoolSize     int           `env:"POOL_SIZE"     env-default:"20"             validate:"min=1,max=100"`
	}

	HTTP struct {
		Host              string        `env:"HOST"                env-default:"0.0.0.0" validate:"required"`
		Port              string        `env:"PORT"                env-default:"8080"    validate:"required"`
		ReadTimeout       time.Duration `env:"READ_TIMEOUT"        env-default:"5s"      validate:"gte=1s,lte=30s"`
		WriteTimeout      time.Duration `env:"WRITE_TIMEOUT"       env-default:"5s"      validate:"gte=1s,lte=30s"`
		IdleTimeout       time.Duration `env:"IDLE_TIMEOUT"        env-default:"60s"     validate:"gte=1s,lte=300s"`
		ShutdownTimeout   time.Duration `env:"SHUTDOWN_TIMEOUT"    env-default:"10s"     validate:"gte=1s,lte=30s"`
		ReadHeaderTimeout time.Duration `env:"READ_HEADER_TIMEOUT" env-default:"5s"      validate:"gte=1s,lte=30s"`
		MaxHeaderBytes    int           `env:"MAX_HEADER_BYTES"    env-default:"1048576" validate:"required,gte=1024,lte=10485760"`
	}

	Logger struct {
		Level      string `env:"LEVEL"       env-default:"info"                  validate:"oneof=debug info warn error"`
		Filename   string `env:"FILENAME"    env-default:"./logs/lshortener.log"`
		MaxSize    int    `env:"MAX_SIZE"    env-default:"100"                   validate:"min=1,max=1000"`
		MaxBackups int    `env:"MAX_BACKUPS" env-default:"3"                     validate:"min=0,max=20"`
		MaxAge     int    `env:"MAX_AGE"     env-default:"28"                    validate:"min=1,max=365"`
	}
)

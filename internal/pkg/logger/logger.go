package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

func New(env string) *zerolog.Logger {
	var logger zerolog.Logger

	if env == "development" {
		output := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
		logger = zerolog.New(output).With().Timestamp().Caller().Logger()
	} else {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	return &logger
}

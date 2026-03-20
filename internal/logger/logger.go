package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	logFormatJSON    = "json"
	logFormatConsole = "console"
)

// Configure sets up zerolog according to LOG_LEVEL (debug, info, warn, error) and LOG_FORMAT (json, console).
func Configure() zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	level := parseLevel(os.Getenv("LOG_LEVEL"))
	writer := selectWriter(os.Getenv("LOG_FORMAT"))

	logger := zerolog.New(writer).Level(level).With().Timestamp().Logger()
	log.Logger = logger
	return logger
}

func parseLevel(value string) zerolog.Level {
	if value == "" {
		return zerolog.InfoLevel
	}

	lvl, err := zerolog.ParseLevel(strings.ToLower(value))
	if err != nil {
		return zerolog.InfoLevel
	}
	return lvl
}

func selectWriter(format string) io.Writer {
	if strings.EqualFold(format, logFormatJSON) {
		return os.Stdout
	}

	return zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}
}

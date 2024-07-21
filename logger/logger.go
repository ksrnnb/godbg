package logger

import (
	"log/slog"
	"os"
)

func NewLogger() *slog.Logger {
	level := slog.LevelInfo
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "DEBUG" {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

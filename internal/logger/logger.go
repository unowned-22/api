package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Log is the singleton logger instance
var Log *logrus.Logger

func Init() {
	Log = logrus.New()

	// Configure JSON Formatter as required
	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})

	Log.SetOutput(os.Stdout)

	// Set logging level from environment or default to info
	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		levelStr = "info"
	}

	level, err := logrus.ParseLevel(levelStr)
	if err != nil {
		level = logrus.InfoLevel
	}
	Log.SetLevel(level)
}

package main

import "github.com/sirupsen/logrus"

// Create a new Logrus logger based on the CLI configuration
func NewLogger(level string, logType string) *logrus.Logger {
	logger := logrus.New()

	switch logType {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{})
	case "console":
		logger.SetFormatter(&logrus.TextFormatter{})
	}

	switch level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	}

	return logger
}

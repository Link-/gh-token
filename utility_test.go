package main

import (
	"testing"

	"github.com/sirupsen/logrus"
)

// Test the NewLogger function
func TestConsoleLogger(t *testing.T) {
	// Create a new logger
	logger := NewLogger("debug", "console")

	// Check the log level
	if logger.Level != logrus.DebugLevel {
		t.Errorf("Log level is not set to debug")
	}

	// Check the log formatter
	if _, ok := logger.Formatter.(*logrus.TextFormatter); !ok {
		t.Errorf("Log formatter is not set to console")
	}
}

func TestJSONLogger(t *testing.T) {
	// Create a new logger
	logger := NewLogger("debug", "json")

	// Check the log level
	if logger.Level != logrus.DebugLevel {
		t.Errorf("Log level is not set to debug")
	}

	// Check the log formatter
	if _, ok := logger.Formatter.(*logrus.JSONFormatter); !ok {
		t.Errorf("Log formatter is not set to JSON")
	}
}

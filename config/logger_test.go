package config_test

import (
	"log/slog"
	"testing"

	"github.com/Logstay/trend-kids-api/config"
	"github.com/stretchr/testify/assert"
)

func TestParseLogLevel(t *testing.T) {
	for in, want := range map[string]slog.Level{
		"": slog.LevelInfo, "info": slog.LevelInfo, "INFO": slog.LevelInfo, " debug ": slog.LevelDebug,
		"warn": slog.LevelWarn, "error": slog.LevelError,
	} {
		got, err := config.ParseLogLevel(in)
		assert.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	_, err := config.ParseLogLevel("verboso")
	assert.Error(t, err)
}

func TestNewLogger_FormatsAndRejectsUnknown(t *testing.T) {
	for _, f := range []string{"", "text", "json"} {
		l, err := config.NewLogger("info", f)
		assert.NoError(t, err, f)
		assert.NotNil(t, l)
	}
	_, err := config.NewLogger("info", "xml")
	assert.Error(t, err)
	_, err = config.NewLogger("verboso", "text")
	assert.Error(t, err)
}

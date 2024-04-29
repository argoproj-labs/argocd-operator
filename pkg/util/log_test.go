package util

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		lvl      string
		expected zapcore.Level
	}{
		{
			"GetLogLevel Error",
			"error",
			zapcore.ErrorLevel,
		},
		{
			"GetLogLevel Warn",
			"warn",
			zapcore.WarnLevel,
		},
		{
			"GetLogLevel Info",
			"info",
			zapcore.InfoLevel,
		},
		{
			"GetLogLevel Debug",
			"debug",
			zapcore.DebugLevel,
		},
		{
			"GetLogLevel Panic",
			"panic",
			zapcore.PanicLevel,
		},
		{
			"GetLogLevel Fatal",
			"fatal",
			zapcore.FatalLevel,
		},
		{
			"GetLogLevel Default",
			"unknown",
			zapcore.InfoLevel,
		},
		{
			"GetLogLevel Mixed Case",
			"DeBuG",
			zapcore.DebugLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLogLevel(tt.lvl)

			if got != tt.expected {
				t.Errorf("GetLogLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

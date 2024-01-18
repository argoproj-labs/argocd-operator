package util

import (
	"strings"

	"go.uber.org/zap/zapcore"
)

func GetLogLevel(lvl string) zapcore.Level {
	switch strings.ToLower(lvl) {
	case "error":
		return zapcore.ErrorLevel
	case "warn":
		return zapcore.WarnLevel
	case "info":
		return zapcore.InfoLevel
	case "debug":
		return zapcore.DebugLevel
	default:
		return zapcore.InfoLevel
	}
}

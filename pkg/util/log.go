package util

import (
	"strings"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Logger struct {
	logr.Logger
}

func NewLogger(name string, keysAndValues ...interface{}) *Logger {
	return &Logger{
		ctrl.Log.WithName(name).WithValues(keysAndValues...),
	}
}

func (logger *Logger) Debug(msg string, keysAndValues ...interface{}) {
	lvl := int(-1 * zapcore.DebugLevel)

	logger.V(lvl).Info(msg, keysAndValues...)
}

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
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

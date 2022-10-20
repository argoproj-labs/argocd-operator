package argoLogger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ctxLogger struct{}

type argoLogger struct {
	z *zap.Logger
}

// ContextWithLogger adds logger to context
func ContextWithLogger(ctx context.Context, l *argoLogger) context.Context {
	return context.WithValue(ctx, ctxLogger{}, l)
}

// LoggerFromContext returns logger from context
func LoggerFromContext(ctx context.Context) *argoLogger {
	if l, ok := ctx.Value(ctxLogger{}).(*argoLogger); ok {
		return l
	}
	return &argoLogger{z: zap.L()}
}

var zapLog *zap.Logger

func (aLog *argoLogger) Info(message string, fields ...zap.Field) {
	aLog.z.Info(message, fields...)
}

func (aLog *argoLogger) Debug(message string, fields ...zap.Field) {
	aLog.z.Debug(message, fields...)
}

func (aLog *argoLogger) Error(message string, fields ...zap.Field) {
	aLog.z.Error(message, fields...)
}

func (aLog *argoLogger) Fatal(message string, fields ...zap.Field) {
	aLog.z.Fatal(message, fields...)
}

func Init() {
	var err error
	config := zap.NewProductionConfig()
	enccoderConfig := zap.NewProductionEncoderConfig()
	enccoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	enccoderConfig.StacktraceKey = "" // to hide stacktrace info
	config.EncoderConfig = enccoderConfig

	zapLog, err = config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(zapLog)
}

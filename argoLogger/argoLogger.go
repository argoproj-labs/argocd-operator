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

type argoField struct {
	key   string
	value interface{}
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

// LoggerWithoutContext returns logger without context
func LoggerWithoutContext() *argoLogger {
	return &argoLogger{z: zap.L()}
}

var zapLog *zap.Logger

func (l *argoLogger) Info(message string, fields ...argoField) {
	l.z.Info(message, getZapFields(fields...)...)
}

func (l *argoLogger) Debug(message string, fields ...argoField) {
	l.z.Debug(message, getZapFields(fields...)...)
}

func (l *argoLogger) Error(message string, fields ...argoField) {
	l.z.Error(message, getZapFields(fields...)...)
}

func (l *argoLogger) Fatal(message string, fields ...argoField) {
	l.z.Fatal(message, getZapFields(fields...)...)
}

func getZapFields(fields ...argoField) []zapcore.Field {
	var zFields []zapcore.Field
	for _, zField := range fields {
		zFields = append(zFields, zap.Any(zField.key, zField.value))
	}
	return zFields
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

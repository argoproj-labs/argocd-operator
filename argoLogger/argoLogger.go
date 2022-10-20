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

// LoggerFromContext returns logger from context
func LoggerWithoutContext() *argoLogger {
	return &argoLogger{z: zap.L()}
}

var zapLog *zap.Logger

func (aLog *argoLogger) Infof(message string, fields ...argoField) {
	aLog.z.Info(message, getZapFields(fields...)...)
}

func (aLog *argoLogger) Debugf(message string, fields ...argoField) {
	aLog.z.Debug(message, getZapFields(fields...)...)
}

func (aLog *argoLogger) Errorf(message string, fields ...argoField) {
	aLog.z.Error(message, getZapFields(fields...)...)
}

func (aLog *argoLogger) Fatalf(message string, fields ...argoField) {
	aLog.z.Fatal(message, getZapFields(fields...)...)
}

func (aLog *argoLogger) Info(message string) {
	aLog.z.Info(message)
}

func (aLog *argoLogger) Debug(message string) {
	aLog.z.Debug(message)
}

func (aLog *argoLogger) Error(message string) {
	aLog.z.Error(message)
}

func (aLog *argoLogger) Fatal(message string) {
	aLog.z.Fatal(message)
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

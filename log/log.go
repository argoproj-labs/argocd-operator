package log

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime/pkg/log"
)

type LogContext struct {
	l logr.Logger
}

func Logger(ctx context.Context) *LogContext {
	logger := &LogContext{l: ctrl.FromContext(ctx)}

	return logger
}

func (logCtx *LogContext) Infof(msg string, args ...interface{}) {
	logCtx.l.Info(fmt.Sprintf(msg, args...))
}

func (logCtx *LogContext) Errorf(err error, msg string, args ...interface{}) {
	logCtx.l.Error(err, fmt.Sprintf(msg, args...))
}

func (logCtx *LogContext) Info(msg string) {
	logCtx.l.Info(fmt.Sprint(msg))
}

func (logCtx *LogContext) Error(err error, msg string) {
	logCtx.l.Error(err, fmt.Sprint(msg))
}

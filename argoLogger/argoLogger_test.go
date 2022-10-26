package argoLogger

import (
	"context"
	"testing"
)

func TestLogger(t *testing.T) {
	ctx := context.Background()

	Init()

	zl := LoggerFromContext(ctx)
	zl.Info("Hello World")
	zl.Info("Hello world", []argoField{{key: "num1", value: 1}, {key: "num2", value: 2}}...)
	ctx = ContextWithLogger(ctx, zl)

}

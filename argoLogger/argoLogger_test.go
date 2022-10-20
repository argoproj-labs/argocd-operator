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
	ctx = ContextWithLogger(ctx, zl)

}

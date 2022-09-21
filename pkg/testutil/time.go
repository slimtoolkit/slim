package testutil

import (
	"context"
	"time"
)

func Delayed(ctx context.Context, delay time.Duration, fn func()) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
		fn()
	}
}

package util

import (
	"context"
	"time"
)

func Delayed(ctx context.Context, delay time.Duration, fn func()) {
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return
	case <-timer.C:
		fn()
	}
}

package util

import (
	"context"
	"log/slog"
	"runtime/debug"
)

// PanicHandler defines an optional failure hook invoked when a goroutine recovers from a panic.
type PanicHandler func(ctx context.Context, workerName string, panicVal any, stackTrace []byte)

// SafeGo spawns an asynchronous task with deferred panic isolation and structured telemetry.
func SafeGo(ctx context.Context, workerName string, fn func(ctx context.Context), onPanic PanicHandler) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				slog.Error("Unhandled panic intercepted in background worker",
					slog.String("worker", workerName),
					slog.Any("panic", r),
					slog.String("stack", string(stack)),
				)
				if onPanic != nil {
					defer func() {
						if rHook := recover(); rHook != nil {
							slog.Error("Panic handler hook itself panicked",
								slog.String("worker", workerName),
								slog.Any("panic", rHook),
							)
						}
					}()
					onPanic(ctx, workerName, r, stack)
				}
			}
		}()
		fn(ctx)
	}()
}

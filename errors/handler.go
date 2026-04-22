package errors

import (
	"context"

	"github.com/chuhongliang/gomelo/lib"
)

type Handler func(ctx *lib.Context) error

func WithErrorHandler(h Handler) Handler {
	return func(ctx *lib.Context) error {
		if err := h(ctx); err != nil {
			if e, ok := err.(*GomeloError); ok {
				ctx.Response(e.ToMap())
				return nil
			}
			ctx.Response(NewResponse(err).ToMap())
			return nil
		}
		return nil
	}
}

type ContextKey string

const ErrorContextKey ContextKey = "gomelo_error"

func WithContext(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, ErrorContextKey, err)
}

func FromContext(ctx context.Context) error {
	if err, ok := ctx.Value(ErrorContextKey).(*GomeloError); ok {
		return err
	}
	return nil
}

func Recover(panicHandler func(interface{})) {
	if r := recover(); r != nil {
		if panicHandler != nil {
			panicHandler(r)
		}
	}
}

func SafeCall(h Handler) (err error) {
	defer Recover(func(r interface{}) {
		err = New(GameError, "panic recovered").(*GomeloError).WithDetail(formatPanic(r))
	})
	return h(nil)
}

func formatPanic(r interface{}) string {
	switch v := r.(type) {
	case error:
		return v.Error()
	case string:
		return v
	default:
		return "unknown panic"
	}
}
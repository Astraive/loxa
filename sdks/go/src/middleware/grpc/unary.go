package grpc

import (
	"context"
	"fmt"

	"github.com/astraive/loxa/sdks/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Config controls gRPC middleware behavior.
type Config struct {
	Event string
}

// UnaryInterceptor starts and emits canonical events around unary handlers.
func UnaryInterceptor(cfg ...Config) grpc.UnaryServerInterceptor {
	c := Config{}
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return UnaryInterceptorWithConfig(c)
}

// UnaryInterceptorWithConfig starts and emits canonical events around unary handlers.
func UnaryInterceptorWithConfig(cfg Config) grpc.UnaryServerInterceptor {
	eventName := cfg.Event
	if eventName == "" {
		eventName = "grpc.request"
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		evCtx := loxa.StartEvent(ctx, loxa.Params{
			Event:  eventName,
			Method: "grpc",
			Route:  info.FullMethod,
			Path:   info.FullMethod,
		})

		if !loxa.PanicRecoveryEnabled() {
			resp, err := handler(evCtx, req)
			if err != nil {
				st, _ := status.FromError(err)
				loxa.FinishError(evCtx, err, loxa.Int("status_code", int(st.Code())))
			} else {
				loxa.Finish(evCtx, "success", loxa.Int("status_code", int(codes.OK)))
			}
			_ = loxa.Emit(evCtx)
			return resp, err
		}

		var resp any
		var err error
		defer func() {
			if rec := recover(); rec != nil {
				err = status.Error(codes.Internal, fmt.Sprintf("panic recovered: %v", rec))
				loxa.FinishError(evCtx, err, loxa.Int("status_code", int(codes.Internal)))
				_ = loxa.Emit(evCtx)
			}
		}()

		resp, err = handler(evCtx, req)
		if err != nil {
			st, _ := status.FromError(err)
			loxa.FinishError(evCtx, err, loxa.Int("status_code", int(st.Code())))
		} else {
			loxa.Finish(evCtx, "success", loxa.Int("status_code", int(codes.OK)))
		}
		_ = loxa.Emit(evCtx)
		return resp, err
	}
}

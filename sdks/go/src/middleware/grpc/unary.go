package grpc

import (
	"context"
	"fmt"

	"github.com/astraive/loza/sdks/go"
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

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		evCtx := loza.StartEvent(ctx, loza.Params{
			Event:  eventName,
			Method: "grpc",
			Route:  info.FullMethod,
			Path:   info.FullMethod,
		})

		if !loza.PanicRecoveryEnabled() {
			resp, err := handler(evCtx, req)
			if err != nil {
				st, _ := status.FromError(err)
				loza.FinishError(evCtx, err, loza.Int("status_code", int(st.Code())))
			} else {
				loza.Finish(evCtx, "success", loza.Int("status_code", int(codes.OK)))
			}
			_ = loza.Emit(evCtx)
			return resp, err
		}

		defer func() {
			if rec := recover(); rec != nil {
				err = status.Error(codes.Internal, fmt.Sprintf("panic recovered: %v", rec))
				loza.FinishError(evCtx, err, loza.Int("status_code", int(codes.Internal)))
				_ = loza.Emit(evCtx)
			}
		}()

		resp, err = handler(evCtx, req)
		if err != nil {
			st, _ := status.FromError(err)
			loza.FinishError(evCtx, err, loza.Int("status_code", int(st.Code())))
		} else {
			loza.Finish(evCtx, "success", loza.Int("status_code", int(codes.OK)))
		}
		_ = loza.Emit(evCtx)
		return resp, err
	}
}

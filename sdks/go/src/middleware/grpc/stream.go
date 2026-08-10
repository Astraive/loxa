package grpc

import (
	"context"
	"fmt"

	"github.com/astraive/loza/sdks/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// StreamInterceptor starts and emits canonical events around stream handlers.
func StreamInterceptor(cfg ...Config) grpc.StreamServerInterceptor {
	c := Config{}
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return StreamInterceptorWithConfig(c)
}

// StreamInterceptorWithConfig starts and emits canonical events around stream handlers.
func StreamInterceptorWithConfig(cfg Config) grpc.StreamServerInterceptor {
	eventName := cfg.Event
	if eventName == "" {
		eventName = "grpc.stream"
	}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		evCtx := loza.StartEvent(ss.Context(), loza.Params{
			Event:  eventName,
			Method: "grpc",
			Route:  info.FullMethod,
			Path:   info.FullMethod,
		})

		wrapped := &wrappedServerStream{ServerStream: ss, ctx: evCtx}
		if !loza.PanicRecoveryEnabled() {
			err := handler(srv, wrapped)
			if err != nil {
				st, _ := status.FromError(err)
				loza.FinishError(evCtx, err, loza.Int("status_code", int(st.Code())))
			} else {
				loza.Finish(evCtx, "success", loza.Int("status_code", int(codes.OK)))
			}
			_ = loza.Emit(evCtx)
			return err
		}

		var err error
		defer func() {
			if rec := recover(); rec != nil {
				err = status.Error(codes.Internal, fmt.Sprintf("panic recovered: %v", rec))
				loza.FinishError(evCtx, err, loza.Int("status_code", int(codes.Internal)))
				_ = loza.Emit(evCtx)
			}
		}()

		err = handler(srv, wrapped)
		if err != nil {
			st, _ := status.FromError(err)
			loza.FinishError(evCtx, err, loza.Int("status_code", int(st.Code())))
		} else {
			loza.Finish(evCtx, "success", loza.Int("status_code", int(codes.OK)))
		}
		_ = loza.Emit(evCtx)
		return err
	}
}

package grpc

import (
	"context"
	"errors"
	"testing"

	loza "github.com/astraive/loza/sdks/go"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeStream struct {
	grpcpkg.ServerStream
	ctx context.Context
}

func (s fakeStream) Context() context.Context { return s.ctx }

func configureGRPC(t *testing.T, panicRecovery bool) *loza.MemorySinkStore {
	t.Helper()
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink).WithPanicRecovery(panicRecovery)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	t.Cleanup(func() { _ = loza.Shutdown(context.Background()) })
	return store
}

func TestUnaryInterceptorEmitsSuccessAndErrorsWithoutRecovery(t *testing.T) {
	store := configureGRPC(t, false)
	info := &grpcpkg.UnaryServerInfo{FullMethod: "/demo.Service/Call"}
	interceptor := UnaryInterceptor()
	resp, err := interceptor(context.Background(), "request", info, func(ctx context.Context, req any) (any, error) {
		if _, ok := loza.FromContext(ctx); !ok {
			t.Error("handler context missing event")
		}
		return "response", nil
	})
	if err != nil || resp != "response" {
		t.Fatalf("success result = %v, %v", resp, err)
	}
	_, err = interceptor(context.Background(), "request", info, func(context.Context, any) (any, error) {
		return nil, status.Error(codes.InvalidArgument, "bad request")
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("error result = %v", err)
	}
	if flushErr := loza.Flush(context.Background()); flushErr != nil {
		t.Fatalf("Flush: %v", flushErr)
	}
	if store.Len() != 2 {
		t.Fatalf("events = %d, want 2", store.Len())
	}
}

func TestUnaryAndStreamInterceptorsRecoverPanics(t *testing.T) {
	store := configureGRPC(t, true)
	info := &grpcpkg.UnaryServerInfo{FullMethod: "/demo.Service/Call"}
	_, err := UnaryInterceptor(Config{Event: "grpc.custom"})(context.Background(), nil, info, func(context.Context, any) (any, error) {
		panic("unary boom")
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("unary panic code = %v", status.Code(err))
	}
	if resp, err := UnaryInterceptor()(context.Background(), nil, info, func(context.Context, any) (any, error) {
		return "ok", nil
	}); err != nil || resp != "ok" {
		t.Fatalf("recovery success = %v, %v", resp, err)
	}
	recoveryErr := status.Error(codes.NotFound, "missing")
	if _, err := UnaryInterceptor()(context.Background(), nil, info, func(context.Context, any) (any, error) {
		return nil, recoveryErr
	}); !errors.Is(err, recoveryErr) {
		t.Fatalf("recovery error = %v", err)
	}
	streamInfo := &grpcpkg.StreamServerInfo{FullMethod: "/demo.Service/Stream"}
	stream := fakeStream{ctx: context.Background()}
	streamErr := StreamInterceptor()(nil, stream, streamInfo, func(_ any, ss grpcpkg.ServerStream) error {
		if _, ok := loza.FromContext(ss.Context()); !ok {
			t.Error("stream context missing event")
		}
		panic("stream boom")
	})
	if status.Code(streamErr) != codes.Internal {
		t.Fatalf("stream panic code = %v", status.Code(streamErr))
	}
	if err := StreamInterceptor()(nil, stream, streamInfo, func(_ any, _ grpcpkg.ServerStream) error { return nil }); err != nil {
		t.Fatalf("stream recovery success: %v", err)
	}
	streamRecoveryErr := status.Error(codes.PermissionDenied, "denied")
	if err := StreamInterceptor()(nil, stream, streamInfo, func(_ any, _ grpcpkg.ServerStream) error {
		return streamRecoveryErr
	}); !errors.Is(err, streamRecoveryErr) {
		t.Fatalf("stream recovery error = %v", err)
	}
	if flushErr := loza.Flush(context.Background()); flushErr != nil {
		t.Fatalf("Flush: %v", flushErr)
	}
	if store.Len() != 6 {
		t.Fatalf("events = %d, want 6", store.Len())
	}
}

func TestStreamInterceptorEmitsSuccessAndError(t *testing.T) {
	store := configureGRPC(t, false)
	streamInfo := &grpcpkg.StreamServerInfo{FullMethod: "/demo.Service/Stream"}
	stream := fakeStream{ctx: context.Background()}
	if err := StreamInterceptor(Config{Event: "grpc.stream.custom"})(nil, stream, streamInfo, func(_ any, _ grpcpkg.ServerStream) error { return nil }); err != nil {
		t.Fatalf("stream success: %v", err)
	}
	wantErr := errors.New("stream failure")
	if err := StreamInterceptor()(nil, stream, streamInfo, func(_ any, _ grpcpkg.ServerStream) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("stream error = %v", err)
	}
	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if store.Len() != 2 {
		t.Fatalf("events = %d, want 2", store.Len())
	}
}

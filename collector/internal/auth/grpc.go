package auth

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCAuthOption configures gRPC auth interceptors.
type GRPCAuthOption func(*grpcAuthConfig)

type grpcAuthConfig struct {
	allowLocalDevKeys bool
}

// GRPCWithAllowLocalDevKeys controls whether lx_local_dev_* keys are accepted.
func GRPCWithAllowLocalDevKeys(v bool) GRPCAuthOption {
	return func(c *grpcAuthConfig) { c.allowLocalDevKeys = v }
}

// UnaryAuthInterceptor returns a gRPC unary server interceptor that validates
// API keys from the "authorization" metadata.
func UnaryAuthInterceptor(store KeyStore, cache *MemoryKeyCache, serverSecret []byte, opts ...GRPCAuthOption) grpc.UnaryServerInterceptor {
	var ac grpcAuthConfig
	for _, o := range opts {
		o(&ac)
	}
	rateLimiter := NewKeyRateLimiter()

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		authCtx, err := grpcAuthenticate(ctx, store, cache, serverSecret, rateLimiter, ac.allowLocalDevKeys)
		if err != nil {
			return nil, err
		}

		ctx = WithAuthContext(ctx, authCtx)
		return handler(ctx, req)
	}
}

// StreamAuthInterceptor returns a gRPC stream server interceptor that validates
// API keys from the "authorization" metadata.
func StreamAuthInterceptor(store KeyStore, cache *MemoryKeyCache, serverSecret []byte, opts ...GRPCAuthOption) grpc.StreamServerInterceptor {
	var ac grpcAuthConfig
	for _, o := range opts {
		o(&ac)
	}
	rateLimiter := NewKeyRateLimiter()

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		authCtx, err := grpcAuthenticate(ss.Context(), store, cache, serverSecret, rateLimiter, ac.allowLocalDevKeys)
		if err != nil {
			return err
		}

		wrapped := &authStream{ServerStream: ss, ctx: WithAuthContext(ss.Context(), authCtx)}
		return handler(srv, wrapped)
	}
}

func grpcAuthenticate(ctx context.Context, store KeyStore, cache *MemoryKeyCache, serverSecret []byte, rateLimiter *KeyRateLimiter, allowLocalDevKeys bool) (*AuthContext, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Extract token from "authorization" or "x-api-key" metadata
	var raw string
	if vals := md.Get("authorization"); len(vals) > 0 {
		auth := vals[0]
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			raw = strings.TrimSpace(auth[7:])
		}
	}
	if raw == "" {
		if vals := md.Get("x-api-key"); len(vals) > 0 {
			raw = strings.TrimSpace(vals[0])
		}
	}
	if raw == "" {
		return nil, status.Error(codes.Unauthenticated, "missing api key")
	}

	// Parse key
	parsed, err := ParseKey(raw)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid api key format")
	}

	// Local dev keys
	if parsed.Kind == KeyKindLocal {
		if !allowLocalDevKeys {
			slog.Warn("local dev key rejected (allow_local_dev_keys=false)", "key", parsed.Raw[:min(len(parsed.Raw), 20)]+"...")
			return nil, status.Error(codes.PermissionDenied, "local dev keys are not allowed in production mode")
		}
		return &AuthContext{
			KeyKind:     KeyKindLocal,
			Permissions: ExpandRoles([]Role{RoleIngestServer}),
			Roles:       []Role{RoleIngestServer},
		}, nil
	}

	// Cache lookup
	var record *KeyRecord
	if cache != nil {
		if cached, ok := cache.Get(parsed.KeyID); ok {
			record = cached
		}
	}

	// Store lookup
	if record == nil {
		record, err = store.FindByKeyID(ctx, parsed.KeyID)
		if err != nil || record == nil {
			if cache != nil {
				cache.SetNegative(parsed.KeyID)
			}
			return nil, status.Error(codes.Unauthenticated, "invalid api key")
		}
		if cache != nil {
			cache.Set(parsed.KeyID, record, 0)
		}
	}

	// Check revoked/expired
	if record.RevokedAt != nil {
		return nil, status.Error(codes.Unauthenticated, "api key revoked")
	}
	if record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt) {
		return nil, status.Error(codes.Unauthenticated, "api key expired")
	}

	// Verify secret
	incomingHash := HashSecret(parsed.Secret, serverSecret)
	if !CompareSecret(incomingHash, record.SecretHash) {
		return nil, status.Error(codes.Unauthenticated, "invalid api key")
	}

	// Build context
	permissions := ExpandRoles(record.Roles)
	ac := &AuthContext{
		OrgID:               record.OrgID,
		ProjectID:           record.ProjectID,
		APIKeyID:            record.KeyID,
		KeyKind:             record.Kind,
		Roles:               record.Roles,
		Permissions:         permissions,
		AllowedEnvs:         record.AllowedEnvs,
		AllowedServices:     record.AllowedServices,
		MaxPayloadBytes:     record.MaxPayloadBytes,
		MaxRequestsPerMinute: record.MaxRequestsPerMinute,
		MaxEventsPerMinute:  record.MaxEventsPerMinute,
		SamplingRate:        record.SamplingRate,
		AllowPII:            record.AllowPII,
		AllowAttachments:    record.AllowAttachments,
	}

	if record.Kind == KeyKindPublic {
		ac.AllowPII = false
		ac.AllowAttachments = false
	}

	// Rate limit
	if rateLimiter != nil && ac.MaxRequestsPerMinute > 0 {
		if !rateLimiter.AllowRequest(ac.APIKeyID, ac.MaxRequestsPerMinute) {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
	}

	return ac, nil
}

// authStream wraps a grpc.ServerStream with a custom context.
type authStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authStream) Context() context.Context {
	return s.ctx
}

package auth

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// GRPCAuthOption configures gRPC auth interceptors.
type GRPCAuthOption func(*grpcAuthConfig)

type grpcAuthConfig struct {
	allowLocalDevKeys bool
	trustedProxies    []*net.IPNet
}

// GRPCWithAllowLocalDevKeys controls whether lz_local_dev_* keys are accepted.
func GRPCWithAllowLocalDevKeys(v bool) GRPCAuthOption {
	return func(c *grpcAuthConfig) { c.allowLocalDevKeys = v }
}

// GRPCWithTrustedProxies sets the list of trusted proxy CIDRs for gRPC IP checks.
func GRPCWithTrustedProxies(proxies []*net.IPNet) GRPCAuthOption {
	return func(c *grpcAuthConfig) { c.trustedProxies = proxies }
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
		authCtx, err := grpcAuthenticate(ctx, store, cache, serverSecret, rateLimiter, ac.allowLocalDevKeys, ac.trustedProxies)
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
		authCtx, err := grpcAuthenticate(ss.Context(), store, cache, serverSecret, rateLimiter, ac.allowLocalDevKeys, ac.trustedProxies)
		if err != nil {
			return err
		}

		wrapped := &authStream{ServerStream: ss, ctx: WithAuthContext(ss.Context(), authCtx)}
		return handler(srv, wrapped)
	}
}

func grpcAuthenticate(ctx context.Context, store KeyStore, cache *MemoryKeyCache, serverSecret []byte, rateLimiter *KeyRateLimiter, allowLocalDevKeys bool, trustedProxies []*net.IPNet) (*AuthContext, error) {
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

	// ABAC checks: AllowedEnvs
	if len(ac.AllowedEnvs) > 0 {
		env := getMetadataValue(md, "x-loza-env")
		if !contains(ac.AllowedEnvs, env) {
			return nil, status.Error(codes.PermissionDenied, "environment not permitted for this key")
		}
	}

	// ABAC checks: AllowedServices
	if len(ac.AllowedServices) > 0 {
		svc := getMetadataValue(md, "x-loza-service")
		if !contains(ac.AllowedServices, svc) {
			return nil, status.Error(codes.PermissionDenied, "service not permitted for this key")
		}
	}

	// ABAC checks: AllowedIPs
	if len(ac.AllowedIPs) > 0 {
		remoteIP := extractIPFromPeer(ctx, trustedProxies)
		if !ipAllowed(remoteIP, ac.AllowedIPs) {
			return nil, status.Error(codes.PermissionDenied, "ip address not permitted")
		}
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

// getMetadataValue returns the first value for the given key from gRPC metadata,
// or "" if not present.
func getMetadataValue(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// extractIPFromPeer extracts the client IP from the gRPC peer address.
// If the peer is a trusted proxy, it checks the x-forwarded-for metadata.
func extractIPFromPeer(ctx context.Context, trustedProxies []*net.IPNet) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil || p.Addr == nil {
		return ""
	}
	addr := p.Addr.String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	remoteIP := net.ParseIP(host)

	// Only trust x-forwarded-for if the direct client is a trusted proxy
	if remoteIP != nil && isTrustedProxy(remoteIP, trustedProxies) {
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			if xff := getMetadataValue(md, "x-forwarded-for"); xff != "" {
				parts := strings.Split(xff, ",")
				for _, part := range parts {
					ip := net.ParseIP(strings.TrimSpace(part))
					if ip != nil && !isTrustedProxy(ip, trustedProxies) {
						return ip.String()
					}
				}
			}
			if xri := getMetadataValue(md, "x-real-ip"); xri != "" {
				return strings.TrimSpace(xri)
			}
		}
	}
	return host
}

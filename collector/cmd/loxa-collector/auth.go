package main

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

var jwtKeyCache sync.Map

func (s *collectorState) isAuthorized(r *http.Request) bool {
	if !s.cfg.authEnabled {
		return true
	}

	mode := strings.ToLower(strings.TrimSpace(s.cfg.identityMode))
	switch mode {
	case "", "payload", "api_key":
		return s.authorizeAPIKey(r)
	case "jwt":
		return s.authorizeJWT(r)
	case "mtls":
		return s.authorizeMTLS(r)
	default:
		return s.authorizeAPIKey(r)
	}
}

func (s *collectorState) authorizeAPIKey(r *http.Request) bool {
	if s.cfg.apiKey == "" {
		return true
	}
	providedKey := strings.TrimSpace(r.Header.Get(s.cfg.apiKeyHeader))
	authorized := subtle.ConstantTimeCompare([]byte(providedKey), []byte(s.cfg.apiKey)) == 1
	if !authorized {
		s.logAuthFailure(r, "api_key")
	}
	return authorized
}

func (s *collectorState) authorizeJWT(r *http.Request) bool {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if authorization == "" {
		s.logAuthFailure(r, "jwt_missing_authorization")
		return false
	}
	if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		s.logAuthFailure(r, "jwt_invalid_authorization_header")
		return false
	}
	token := strings.TrimSpace(authorization[len("Bearer "):])
	if token == "" {
		s.logAuthFailure(r, "jwt_empty_token")
		return false
	}
	key := jwtVerificationKey(strings.TrimSpace(s.cfg.apiKey))
	if key == nil {
		s.logAuthFailure(r, "jwt_verification_key_unconfigured")
		return false
	}

	parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{
		jose.HS256, jose.HS384, jose.HS512, jose.RS256, jose.RS384, jose.RS512, jose.ES256, jose.ES384, jose.ES512,
	})
	if err != nil {
		s.logAuthFailure(r, "jwt_parse_failed")
		return false
	}

	claims := jwt.Claims{}
	if err := parsed.Claims(key, &claims); err != nil {
		s.logAuthFailure(r, "jwt_claims_failed")
		return false
	}
	if err := claims.Validate(jwt.Expected{Time: time.Now()}); err != nil {
		s.logAuthFailure(r, "jwt_validation_failed")
		return false
	}
	return true
}

func (s *collectorState) authorizeMTLS(r *http.Request) bool {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		s.logAuthFailure(r, "mtls_missing_client_certificate")
		return false
	}
	for _, cert := range r.TLS.PeerCertificates {
		if cert != nil {
			return true
		}
	}
	s.logAuthFailure(r, "mtls_invalid_client_certificate")
	return false
}

func (s *collectorState) logAuthFailure(r *http.Request, reason string) {
	logJSON("warn", "collector_auth_failed", map[string]any{
		"path":        r.URL.Path,
		"method":      r.Method,
		"remote_addr": r.RemoteAddr,
		"reason":      reason,
	})
}

func jwtVerificationKey(raw string) any {
	if raw == "" {
		return nil
	}
	if cached, ok := jwtKeyCache.Load(raw); ok {
		return cached
	}

	key := parseJWTVerificationKey(raw)
	jwtKeyCache.Store(raw, key)
	return key
}

func parseJWTVerificationKey(raw string) any {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return []byte(raw)
	}

	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		switch pk := cert.PublicKey.(type) {
		case *rsa.PublicKey:
			return pk
		case *ecdsa.PublicKey:
			return pk
		}
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		switch typed := pub.(type) {
		case *rsa.PublicKey:
			return typed
		case *ecdsa.PublicKey:
			return typed
		}
	}
	if certs, err := x509.ParseCertificates(block.Bytes); err == nil && len(certs) > 0 {
		switch pk := certs[0].PublicKey.(type) {
		case *rsa.PublicKey:
			return pk
		case *ecdsa.PublicKey:
			return pk
		}
	}
	return []byte(raw)
}

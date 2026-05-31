package main

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

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
		s.logAuthFailure(r, "api_key_unconfigured")
		return false
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

	parsed, err := jwt.ParseSigned(token, jwtSignatureAlgorithms(key))
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
	hasCNAllowlist := len(s.cfg.mtlsAllowedCNs) > 0
	hasDNSAllowlist := len(s.cfg.mtlsAllowedDNS) > 0
	hasEmailAllowlist := len(s.cfg.mtlsAllowedEmails) > 0
	hasAnyAllowlist := hasCNAllowlist || hasDNSAllowlist || hasEmailAllowlist

	for _, cert := range r.TLS.PeerCertificates {
		if cert == nil {
			continue
		}
		cn := cert.Subject.CommonName
		// If no allowlists configured, accept any cert with non-empty CN (backward compat)
		if !hasAnyAllowlist {
			if cn != "" {
				return true
			}
			continue
		}
		// Check CN against allowlist
		if hasCNAllowlist && cn != "" {
			for _, allowed := range s.cfg.mtlsAllowedCNs {
				if cn == allowed {
					return true
				}
			}
		}
		// Check DNS names against allowlist
		if hasDNSAllowlist {
			for _, dns := range cert.DNSNames {
				for _, allowed := range s.cfg.mtlsAllowedDNS {
					if dns == allowed {
						return true
					}
				}
			}
		}
		// Check email addresses against allowlist
		if hasEmailAllowlist {
			for _, email := range cert.EmailAddresses {
				for _, allowed := range s.cfg.mtlsAllowedEmails {
					if email == allowed {
						return true
					}
				}
			}
		}
	}
	s.logAuthFailure(r, "mtls_no_matching_identity")
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
	return parseJWTVerificationKey(raw)
}

func jwtSignatureAlgorithms(key any) []jose.SignatureAlgorithm {
	switch key.(type) {
	case []byte:
		return []jose.SignatureAlgorithm{jose.HS256, jose.HS384, jose.HS512}
	case *rsa.PublicKey:
		return []jose.SignatureAlgorithm{jose.RS256, jose.RS384, jose.RS512}
	case *ecdsa.PublicKey:
		return []jose.SignatureAlgorithm{jose.ES256, jose.ES384, jose.ES512}
	default:
		return nil
	}
}

// extractCryptoKey returns the key if it's an RSA or ECDSA public key, nil otherwise.
func extractCryptoKey(key any) any {
	switch k := key.(type) {
	case *rsa.PublicKey:
		return k
	case *ecdsa.PublicKey:
		return k
	}
	return nil
}

func parseJWTVerificationKey(raw string) any {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return []byte(raw)
	}

	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		if k := extractCryptoKey(cert.PublicKey); k != nil {
			return k
		}
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if k := extractCryptoKey(pub); k != nil {
			return k
		}
	}
	if certs, err := x509.ParseCertificates(block.Bytes); err == nil && len(certs) > 0 {
		if k := extractCryptoKey(certs[0].PublicKey); k != nil {
			return k
		}
	}
	return []byte(raw)
}

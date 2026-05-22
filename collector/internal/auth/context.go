package auth

import "context"

type authContextKey struct{}

// AuthContext holds the authenticated identity and authorization details
// for a request. Attached to the request context by the auth middleware.
type AuthContext struct {
	OrgID     string
	ProjectID string
	APIKeyID  string
	KeyKind   KeyKind

	Roles       []Role
	Permissions map[Permission]bool

	AllowedEnvs         []string
	AllowedServices     []string
	AllowedOrigins      []string
	AllowedIPs          []string
	MaxPayloadBytes     int
	MaxRequestsPerMinute int
	MaxEventsPerMinute  int
	SamplingRate        float64
	AllowPII            bool
	AllowAttachments    bool
}

// WithAuthContext attaches an AuthContext to the context.
func WithAuthContext(ctx context.Context, ac *AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, ac)
}

// GetAuthContext retrieves the AuthContext from the context.
// Returns nil if no auth context is present.
func GetAuthContext(ctx context.Context) *AuthContext {
	ac, _ := ctx.Value(authContextKey{}).(*AuthContext)
	return ac
}

// HasPermission checks if the auth context includes a specific permission.
func (ac *AuthContext) HasPermission(p Permission) bool {
	if ac == nil {
		return false
	}
	return ac.Permissions[p]
}

// HasAnyPermission checks if the auth context includes any of the given permissions.
func (ac *AuthContext) HasAnyPermission(perms ...Permission) bool {
	if ac == nil {
		return false
	}
	for _, p := range perms {
		if ac.Permissions[p] {
			return true
		}
	}
	return false
}

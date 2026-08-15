package auth

import "context"

type authContextKey struct{}

// CollectorGrant binds a principal to one named collector, its permitted
// environments, and the collector-scoped actions it may perform.
type CollectorGrant struct {
	Collector    string
	Environments []string
	Permissions  map[Permission]bool
}

// permits reports whether this grant authorizes a specific collector request.
// Empty collector or environment values are denied so callers cannot bypass
// scope resolution by omitting one of the resource dimensions.
func (g CollectorGrant) permits(collector, environment string, permission Permission) bool {
	if collector == "" || environment == "" || g.Collector != collector || !g.Permissions[permission] {
		return false
	}
	return contains(g.Environments, environment)
}

func cloneCollectorGrants(grants []CollectorGrant) []CollectorGrant {
	if len(grants) == 0 {
		return nil
	}
	cloned := make([]CollectorGrant, len(grants))
	for i, grant := range grants {
		cloned[i] = CollectorGrant{
			Collector:    grant.Collector,
			Environments: append([]string(nil), grant.Environments...),
			Permissions:  make(map[Permission]bool, len(grant.Permissions)),
		}
		for permission, allowed := range grant.Permissions {
			cloned[i].Permissions[permission] = allowed
		}
	}
	return cloned
}

// AuthContext holds the authenticated identity and authorization details
// for a request. Attached to the request context by the auth middleware.
type AuthContext struct {
	OrgID     string
	ProjectID string
	APIKeyID  string
	KeyKind   KeyKind

	Roles           []Role
	Permissions     map[Permission]bool
	CollectorGrants []CollectorGrant

	AllowedEnvs          []string
	AllowedServices      []string
	AllowedOrigins       []string
	AllowedIPs           []string
	MaxPayloadBytes      int
	MaxRequestsPerMinute int
	MaxEventsPerMinute   int
	SamplingRate         float64
	AllowPII             bool
	AllowAttachments     bool
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

// AuthorizesCollector verifies the complete server-resolved resource scope.
// A principal without a matching collector grant is denied even when it holds
// the underlying global permission. Role-bound credentials must satisfy both
// their role and their resource scope; roleless legacy grants retain their
// explicitly configured compatibility policy.
func (ac *AuthContext) AuthorizesCollector(collector, environment string, permission Permission) bool {
	if ac == nil {
		return false
	}
	if len(ac.Roles) > 0 && !ac.HasPermission(permission) {
		return false
	}
	for _, grant := range ac.CollectorGrants {
		if grant.permits(collector, environment, permission) {
			return true
		}
	}
	return false
}

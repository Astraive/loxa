package auth

// Permission represents a specific API capability.
type Permission string

const (
	PermEventsWrite    Permission = "events:write"
	PermEventsRead     Permission = "events:read"
	PermEventsDelete   Permission = "events:delete"
	PermLogsWrite      Permission = "logs:write"
	PermLogsRead       Permission = "logs:read"
	PermLogsEdit       Permission = "logs:edit"
	PermLogsDelete     Permission = "logs:delete"
	PermTracesWrite    Permission = "traces:write"
	PermTracesRead     Permission = "traces:read"
	PermMetricsWrite   Permission = "metrics:write"
	PermMetricsRead    Permission = "metrics:read"
	PermHeartbeatWrite Permission = "heartbeat:write"
	PermSchemaWrite    Permission = "schema:write"
	PermSchemaRead     Permission = "schema:read"
	PermPIIAuditRead   Permission = "pii_audit:read"
	PermStatusRead     Permission = "status:read"
	PermProjectAdmin   Permission = "project:admin"
)

// AccessMode describes whether a credential can be exposed to an untrusted
// client. Public credentials are intentionally restricted to RoleClient.
type AccessMode string

const (
	ModePublic  AccessMode = "public"
	ModePrivate AccessMode = "private"
)

// Role represents a named bundle of permissions.
type Role string

const (
	RoleIngestPublic     Role = "collector_ingest_public"
	RoleIngestServer     Role = "collector_ingest_server"
	RoleIngestEnterprise Role = "collector_ingest_enterprise"
	RoleProjectReadonly  Role = "project_readonly"
	RoleProjectOperator  Role = "project_operator"
	RoleProjectAdmin     Role = "project_admin"
	RoleUser             Role = "user"
	RoleClient           Role = "client"
	RoleAdmin            Role = "admin"
	RoleSuperAdmin       Role = "superadmin"
)

// rolePermissions maps each role to its allowed permissions.
// Default-deny: unknown roles get no permissions.
var rolePermissions = map[Role][]Permission{
	RoleUser: {
		PermLogsRead,
	},
	RoleClient: {
		PermLogsRead,
		PermLogsWrite,
	},
	RoleAdmin: {
		PermLogsRead,
		PermLogsWrite,
		PermLogsEdit,
		PermLogsDelete,
	},
	RoleSuperAdmin: {
		PermLogsRead,
		PermLogsWrite,
		PermLogsEdit,
		PermLogsDelete,
		PermProjectAdmin,
	},
	RoleIngestPublic: {
		PermEventsWrite,
		PermHeartbeatWrite,
	},
	RoleIngestServer: {
		PermEventsWrite,
		PermLogsWrite,
		PermTracesWrite,
		PermMetricsWrite,
		PermHeartbeatWrite,
	},
	RoleIngestEnterprise: {
		PermEventsWrite,
		PermLogsWrite,
		PermTracesWrite,
		PermMetricsWrite,
		PermHeartbeatWrite,
	},
	RoleProjectReadonly: {
		PermEventsRead,
		PermLogsRead,
		PermTracesRead,
		PermMetricsRead,
		PermSchemaRead,
		PermPIIAuditRead,
		PermStatusRead,
	},
	RoleProjectOperator: {
		PermEventsRead,
		PermLogsRead,
		PermTracesRead,
		PermMetricsRead,
		PermSchemaRead,
		PermSchemaWrite,
		PermPIIAuditRead,
		PermStatusRead,
	},
	RoleProjectAdmin: {
		PermEventsRead,
		PermEventsDelete,
		PermLogsRead,
		PermTracesRead,
		PermMetricsRead,
		PermSchemaRead,
		PermSchemaWrite,
		PermPIIAuditRead,
		PermProjectAdmin,
		PermStatusRead,
		// Note: project_admin does NOT include ingest permissions.
		// Ingest keys and admin keys are separate concerns.
	},
}

// HasPermission checks if a role includes a specific permission.
func (r Role) HasPermission(p Permission) bool {
	perms, ok := rolePermissions[r]
	if !ok {
		return false // unknown role = deny
	}
	for _, pp := range perms {
		if pp == p {
			return true
		}
	}
	return false
}

// ExpandRoles returns the union of all permissions for the given roles.
func ExpandRoles(roles []Role) map[Permission]bool {
	result := make(map[Permission]bool)
	for _, r := range roles {
		for _, p := range rolePermissions[r] {
			result[p] = true
		}
	}
	return result
}

// IsKnownRole reports whether role resolves to an explicit, default-deny
// permission bundle.
func IsKnownRole(role Role) bool {
	_, ok := rolePermissions[role]
	return ok
}

// RoleAllowedInMode keeps bearer credentials safe for browser use: public
// credentials may grant only the least-privileged client role. Private
// credentials may use every known role.
func RoleAllowedInMode(role Role, mode AccessMode) bool {
	switch mode {
	case ModePublic:
		return role == RoleClient
	case ModePrivate:
		return IsKnownRole(role)
	default:
		return false
	}
}

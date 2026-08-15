package auth

import "testing"

func TestProductRolesExposeLogCapabilities(t *testing.T) {
	tests := []struct {
		role       Role
		allowed    []Permission
		notAllowed []Permission
	}{
		{RoleUser, []Permission{PermLogsRead}, []Permission{PermLogsWrite, PermLogsEdit, PermLogsDelete}},
		{RoleClient, []Permission{PermLogsRead, PermLogsWrite}, []Permission{PermLogsEdit, PermLogsDelete}},
		{RoleAdmin, []Permission{PermLogsRead, PermLogsWrite, PermLogsEdit, PermLogsDelete}, nil},
		{RoleSuperAdmin, []Permission{PermLogsRead, PermLogsWrite, PermLogsEdit, PermLogsDelete, PermProjectAdmin}, nil},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			for _, permission := range tt.allowed {
				if !tt.role.HasPermission(permission) {
					t.Errorf("%s must permit %s", tt.role, permission)
				}
			}
			for _, permission := range tt.notAllowed {
				if tt.role.HasPermission(permission) {
					t.Errorf("%s must not permit %s", tt.role, permission)
				}
			}
		})
	}
}

func TestPublicModeRejectsPrivilegedRoles(t *testing.T) {
	if !RoleAllowedInMode(RoleClient, ModePublic) {
		t.Fatal("client must be usable by public credentials")
	}
	for _, role := range []Role{RoleUser, RoleAdmin, RoleSuperAdmin} {
		if RoleAllowedInMode(role, ModePublic) {
			t.Errorf("public mode must reject %s", role)
		}
	}
	if !RoleAllowedInMode(RoleSuperAdmin, ModePrivate) {
		t.Fatal("private mode must support superadmin")
	}
}

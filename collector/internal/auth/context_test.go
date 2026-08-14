package auth

import "testing"

func TestAuthContextAuthorizesCollectorOnlyWithinGrantScope(t *testing.T) {
	if (&AuthContext{}).AuthorizesCollector("payments", "prod", PermEventsWrite) {
		t.Fatal("principal without collector binding must be denied")
	}

	context := &AuthContext{
		CollectorGrants: []CollectorGrant{{
			Collector:    "payments",
			Environments: []string{"prod"},
			Permissions: map[Permission]bool{
				PermEventsWrite: true,
			},
		}},
	}

	if !context.AuthorizesCollector("payments", "prod", PermEventsWrite) {
		t.Fatal("expected matching collector grant to authorize request")
	}
	if context.AuthorizesCollector("analytics", "prod", PermEventsWrite) {
		t.Fatal("cross-collector request must be denied")
	}
	if context.AuthorizesCollector("payments", "dev", PermEventsWrite) {
		t.Fatal("ungranted environment must be denied")
	}
	if context.AuthorizesCollector("payments", "", PermEventsWrite) {
		t.Fatal("missing environment must be denied")
	}
	if context.AuthorizesCollector("payments", "prod", PermEventsRead) {
		t.Fatal("ungranted permission must be denied")
	}
}

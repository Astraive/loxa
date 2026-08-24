package env

import "testing"

func TestGetStringUsesValueAndDefault(t *testing.T) {
	t.Setenv("LOZA_ENV_STRING", "configured")
	if got := GetString("LOZA_ENV_STRING", "fallback"); got != "configured" {
		t.Fatalf("GetString configured = %q, want configured", got)
	}
	if got := GetString("LOZA_ENV_STRING_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("GetString missing = %q, want fallback", got)
	}
	t.Setenv("LOZA_ENV_STRING_EMPTY", "")
	if got := GetString("LOZA_ENV_STRING_EMPTY", "fallback"); got != "fallback" {
		t.Fatalf("GetString empty = %q, want fallback", got)
	}
}

func TestGetBoolParsesValueAndFallsBack(t *testing.T) {
	t.Setenv("LOZA_ENV_BOOL_TRUE", "true")
	if got := GetBool("LOZA_ENV_BOOL_TRUE", false); !got {
		t.Fatal("GetBool true = false")
	}
	t.Setenv("LOZA_ENV_BOOL_FALSE", "0")
	if got := GetBool("LOZA_ENV_BOOL_FALSE", true); got {
		t.Fatal("GetBool false = true")
	}
	for _, tc := range []struct {
		name string
		value string
		def  bool
	}{
		{name: "missing", def: true},
		{name: "empty", value: "", def: true},
		{name: "invalid", value: "not-bool", def: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LOZA_ENV_BOOL_CASE", tc.value)
			if tc.name == "missing" {
				t.Setenv("LOZA_ENV_BOOL_CASE", "")
			}
			if got := GetBool("LOZA_ENV_BOOL_CASE", tc.def); got != tc.def {
				t.Fatalf("GetBool fallback = %v, want %v", got, tc.def)
			}
		})
	}
}

func TestGetIntParsesValueAndFallsBack(t *testing.T) {
	t.Setenv("LOZA_ENV_INT", "42")
	if got := GetInt("LOZA_ENV_INT", 7); got != 42 {
		t.Fatalf("GetInt configured = %d, want 42", got)
	}
	for _, tc := range []struct {
		name string
		value string
		def  int
	}{
		{name: "missing", def: 7},
		{name: "empty", value: "", def: 8},
		{name: "invalid", value: "not-int", def: 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "missing" {
				t.Setenv("LOZA_ENV_INT_CASE", "")
			} else {
				t.Setenv("LOZA_ENV_INT_CASE", tc.value)
			}
			if got := GetInt("LOZA_ENV_INT_CASE", tc.def); got != tc.def {
				t.Fatalf("GetInt fallback = %d, want %d", got, tc.def)
			}
		})
	}
}

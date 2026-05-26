package config

import (
	"encoding/json"
	"testing"
)

func TestDepTargetJSONRef(t *testing.T) {
	data := []byte(`"auth_proxy"`)

	var target DepTarget
	if err := json.Unmarshal(data, &target); err != nil {
		t.Fatalf("Unmarshal ref: %v", err)
	}
	if target.Ref != "auth_proxy" {
		t.Fatalf("Ref = %q, want auth_proxy", target.Ref)
	}
	if target.Inline != nil {
		t.Fatal("Inline is set for ref target")
	}

	out, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal ref: %v", err)
	}
	if string(out) != `"auth_proxy"` {
		t.Fatalf("Marshal ref = %s, want \"auth_proxy\"", string(out))
	}
}

func TestDepTargetJSONInline(t *testing.T) {
	data := []byte(`{
		"type": "Proxy",
		"name": "inline_audit",
		"handlers": [{
			"type": "Replier",
			"category": "audit",
			"socket": {"id": "audit_1", "port": 4301}
		}]
	}`)

	var target DepTarget
	if err := json.Unmarshal(data, &target); err != nil {
		t.Fatalf("Unmarshal inline: %v", err)
	}
	if target.Ref != "" {
		t.Fatalf("Ref = %q, want empty", target.Ref)
	}
	if target.Inline == nil || target.Inline.Name != "inline_audit" {
		t.Fatalf("Inline service = %#v, want inline_audit", target.Inline)
	}

	out, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal inline: %v", err)
	}

	var roundTrip DepTarget
	if err := json.Unmarshal(out, &roundTrip); err != nil {
		t.Fatalf("Unmarshal round trip: %v", err)
	}
	if roundTrip.Name() != "inline_audit" {
		t.Fatalf("Name() = %q, want inline_audit", roundTrip.Name())
	}
}

func TestDepTargetValidate(t *testing.T) {
	if err := ValidateDepTarget(DepTarget{}); err == nil {
		t.Fatal("ValidateDepTarget empty returned nil error")
	}
	if err := ValidateDepTarget(DepTarget{Ref: "a", Inline: New("b", ProxyType)}); err == nil {
		t.Fatal("ValidateDepTarget ref and inline returned nil error")
	}
	if err := ValidateDepTarget(RefTarget("auth_proxy")); err != nil {
		t.Fatalf("ValidateDepTarget ref: %v", err)
	}
}

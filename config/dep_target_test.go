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
	if target.Proxy != nil {
		t.Fatal("Proxy is set for ref target")
	}

	out, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal ref: %v", err)
	}
	if string(out) != `"auth_proxy"` {
		t.Fatalf("Marshal ref = %s, want \"auth_proxy\"", string(out))
	}
}

func TestDepTargetJSONInlineService(t *testing.T) {
	data := []byte(`{
		"type": "Extension",
		"name": "inline_worker",
		"handlers": [{
			"type": "Replier",
			"category": "worker",
			"endpoint": {"id": "worker_1", "port": 4301}
		}]
	}`)

	var target DepTarget
	if err := json.Unmarshal(data, &target); err != nil {
		t.Fatalf("Unmarshal inline: %v", err)
	}
	if target.Ref != "" {
		t.Fatalf("Ref = %q, want empty", target.Ref)
	}
	if target.Inline == nil || target.Inline.Name != "inline_worker" {
		t.Fatalf("Inline service = %#v, want inline_worker", target.Inline)
	}
	if target.Proxy != nil {
		t.Fatalf("Proxy = %#v, want nil", target.Proxy)
	}

	out, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal inline: %v", err)
	}

	var roundTrip DepTarget
	if err := json.Unmarshal(out, &roundTrip); err != nil {
		t.Fatalf("Unmarshal round trip: %v", err)
	}
	if roundTrip.Name() != "inline_worker" {
		t.Fatalf("Name() = %q, want inline_worker", roundTrip.Name())
	}
}

func TestDepTargetJSONInlineProxy(t *testing.T) {
	data := []byte(`{
		"type": "Proxy",
		"name": "inline_audit",
		"handlers": [{
			"type": "Replier",
			"category": "audit",
			"endpoint": {"id": "audit_1", "port": 4301},
			"routes": ["audit"],
			"outbounds": ["audit_sink"]
		}]
	}`)

	var target DepTarget
	if err := json.Unmarshal(data, &target); err != nil {
		t.Fatalf("Unmarshal inline proxy: %v", err)
	}
	if target.Ref != "" {
		t.Fatalf("Ref = %q, want empty", target.Ref)
	}
	if target.Inline != nil {
		t.Fatalf("Inline service = %#v, want nil", target.Inline)
	}
	if target.Proxy == nil || target.Proxy.Name != "inline_audit" {
		t.Fatalf("Proxy = %#v, want inline_audit", target.Proxy)
	}
	if len(target.Proxy.Handlers) != 1 || len(target.Proxy.Handlers[0].Outbounds) != 1 {
		t.Fatalf("Proxy handlers = %#v, want one handler with one outbound", target.Proxy.Handlers)
	}

	out, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal inline proxy: %v", err)
	}

	var roundTrip DepTarget
	if err := json.Unmarshal(out, &roundTrip); err != nil {
		t.Fatalf("Unmarshal round trip: %v", err)
	}
	if roundTrip.Proxy == nil || roundTrip.Proxy.Name != "inline_audit" {
		t.Fatalf("round trip proxy = %#v, want inline_audit", roundTrip.Proxy)
	}
}

func TestDepTargetValidate(t *testing.T) {
	if err := ValidateDepTarget(DepTarget{}); err == nil {
		t.Fatal("ValidateDepTarget empty returned nil error")
	}
	if err := ValidateDepTarget(DepTarget{Ref: "a", Inline: New("b", ProxyType)}); err == nil {
		t.Fatal("ValidateDepTarget ref and inline returned nil error")
	}
	if err := ValidateDepTarget(DepTarget{Inline: New("b", ProxyType), Proxy: &Proxy{Service: *New("c", ProxyType)}}); err == nil {
		t.Fatal("ValidateDepTarget inline and proxy returned nil error")
	}
	if err := ValidateDepTarget(RefTarget("auth_proxy")); err != nil {
		t.Fatalf("ValidateDepTarget ref: %v", err)
	}
}

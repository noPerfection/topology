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
		t.Fatalf("Path = %q, want auth_proxy", target.Ref)
	}
	service, category := target.RefPath()
	if service != "auth_proxy" || category != "" {
		t.Fatalf("Ref() = (%q, %q), want (auth_proxy, \"\")", service, category)
	}
	if target.Name() != "auth_proxy" {
		t.Fatalf("Name() = %q, want auth_proxy", target.Name())
	}
	if !target.Service.IsZero() {
		t.Fatal("Service is set for ref target")
	}

	out, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal ref: %v", err)
	}
	if string(out) != `"auth_proxy"` {
		t.Fatalf("Marshal ref = %s, want \"auth_proxy\"", string(out))
	}
}

func TestDepTargetJSONRefWithHandler(t *testing.T) {
	data := []byte(`"auth_proxy/main"`)

	var target DepTarget
	if err := json.Unmarshal(data, &target); err != nil {
		t.Fatalf("Unmarshal ref: %v", err)
	}
	service, category := target.RefPath()
	if service != "auth_proxy" || category != "main" {
		t.Fatalf("Ref() = (%q, %q), want (auth_proxy, main)", service, category)
	}
	if target.Name() != "auth_proxy" {
		t.Fatalf("Name() = %q, want auth_proxy", target.Name())
	}

	out, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal ref path: %v", err)
	}
	if string(out) != `"auth_proxy/main"` {
		t.Fatalf("Marshal ref path = %s, want \"auth_proxy/main\"", string(out))
	}
}

func TestDepTargetRefPathInvalid(t *testing.T) {
	for _, ref := range []string{"", "service_name/", "/main", "service_name//main"} {
		if err := ValidateDepTarget(DepTarget{Ref: ref}); err == nil {
			t.Fatalf("ValidateDepTarget(%q) returned nil error", ref)
		}

		data, err := json.Marshal(ref)
		if err != nil {
			t.Fatalf("Marshal test ref %q: %v", ref, err)
		}
		var target DepTarget
		if err := json.Unmarshal(data, &target); err == nil {
			t.Fatalf("Unmarshal invalid ref %q returned nil error", ref)
		}
	}
}

func TestRefTargetBuilder(t *testing.T) {
	serviceOnly := RefTarget("auth_proxy")
	service, category := serviceOnly.RefPath()
	if service != "auth_proxy" || category != "" {
		t.Fatalf("service-only Ref() = (%q, %q), want (auth_proxy, \"\")", service, category)
	}
	if serviceOnly.Ref != "auth_proxy" {
		t.Fatalf("service-only path = %q, want auth_proxy", serviceOnly.Ref)
	}

	target := RefTarget("auth_proxy", "main")
	service, category = target.RefPath()
	if service != "auth_proxy" || category != "main" {
		t.Fatalf("Ref() = (%q, %q), want (auth_proxy, main)", service, category)
	}
	if target.Ref != "auth_proxy/main" {
		t.Fatalf("stored path = %q, want auth_proxy/main", target.Ref)
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
		t.Fatalf("Path = %q, want empty", target.Ref)
	}
	if target.Service.Name != "inline_worker" {
		t.Fatalf("Service = %#v, want inline_worker", target.Service)
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
		t.Fatalf("Path = %q, want empty", target.Ref)
	}
	if target.Type != ProxyType {
		t.Fatalf("Service = %#v, want proxy", target.Service)
	}
	if target.Service.Name != "inline_audit" {
		t.Fatalf("Service = %#v, want inline_audit", target.Service)
	}
	if len(target.Handlers) != 1 || len(target.Handlers[0].AsProxyHandler().Outbounds) != 1 {
		t.Fatalf("Proxy handlers = %#v, want one handler with one outbound", target.Handlers)
	}

	out, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal inline proxy: %v", err)
	}

	var roundTrip DepTarget
	if err := json.Unmarshal(out, &roundTrip); err != nil {
		t.Fatalf("Unmarshal round trip: %v", err)
	}
	if roundTrip.Service.Name != "inline_audit" {
		t.Fatalf("round trip proxy = %#v, want inline_audit", roundTrip.Service)
	}
}

func TestDepTargetValidate(t *testing.T) {
	if err := ValidateDepTarget(DepTarget{}); err == nil {
		t.Fatal("ValidateDepTarget empty returned nil error")
	}
	if err := ValidateDepTarget(DepTarget{Ref: "a", Service: Service{Name: "b", Type: ProxyType}}); err == nil {
		t.Fatal("ValidateDepTarget ref and service returned nil error")
	}
	if err := ValidateDepTarget(DepTarget{Service: Service{}}); err == nil {
		t.Fatal("ValidateDepTarget empty service returned nil error")
	}
	if err := ValidateDepTarget(RefTarget("auth_proxy")); err != nil {
		t.Fatalf("ValidateDepTarget ref: %v", err)
	}
}

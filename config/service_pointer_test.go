package config

import (
	"encoding/json"
	"testing"

	"github.com/noPerfection/protocol/message"
)

func TestServicePointerJSONRef(t *testing.T) {
	data := []byte(`"auth_proxy"`)

	var target ServicePointer
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

func TestServicePointerJSONRefWithHandler(t *testing.T) {
	data := []byte(`"auth_proxy/main"`)

	var target ServicePointer
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

func TestServicePointerRefPathInvalid(t *testing.T) {
	for _, ref := range []string{"", "service_name/", "/main", "service_name//main"} {
		if err := ValidateServicePointer(ServicePointer{Ref: ref}); err == nil {
			t.Fatalf("ValidateServicePointer(%q) returned nil error", ref)
		}

		data, err := json.Marshal(ref)
		if err != nil {
			t.Fatalf("Marshal test ref %q: %v", ref, err)
		}
		var target ServicePointer
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

func TestServicePointerJSONInlineService(t *testing.T) {
	data := []byte(`{
		"type": "Extension",
		"name": "inline_worker",
		"handlers": [{
			"type": "Replier",
			"category": "worker",
			"endpoint": {"id": "worker_1", "port": 4301}
		}]
	}`)

	var target ServicePointer
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

	var roundTrip ServicePointer
	if err := json.Unmarshal(out, &roundTrip); err != nil {
		t.Fatalf("Unmarshal round trip: %v", err)
	}
	if roundTrip.Name() != "inline_worker" {
		t.Fatalf("Name() = %q, want inline_worker", roundTrip.Name())
	}
}

func TestServicePointerJSONInlineProxy(t *testing.T) {
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

	var target ServicePointer
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

	var roundTrip ServicePointer
	if err := json.Unmarshal(out, &roundTrip); err != nil {
		t.Fatalf("Unmarshal round trip: %v", err)
	}
	if roundTrip.Service.Name != "inline_audit" {
		t.Fatalf("round trip proxy = %#v, want inline_audit", roundTrip.Service)
	}
}

func TestServicePointerValidate(t *testing.T) {
	if err := ValidateServicePointer(ServicePointer{}); err == nil {
		t.Fatal("ValidateServicePointer empty returned nil error")
	}
	if err := ValidateServicePointer(ServicePointer{Ref: "a", Service: Service{Name: "b", Type: ProxyType}}); err == nil {
		t.Fatal("ValidateServicePointer ref and service returned nil error")
	}
	if err := ValidateServicePointer(ServicePointer{Service: Service{}}); err == nil {
		t.Fatal("ValidateServicePointer empty service returned nil error")
	}
	if err := ValidateServicePointer(RefTarget("auth_proxy")); err != nil {
		t.Fatalf("ValidateServicePointer ref: %v", err)
	}
}

func TestServicePointerInlineIpcRequiresCompleteService(t *testing.T) {
	inline := ServiceTarget(Service{
		Type: ProxyType,
		Name: "inline_proxy",
		Handlers: NewHandlerVariants(Handler{
			Type:     SyncReplierType,
			Category: "main",
			Endpoint: message.NewEndpoint("tmp/inline_proxy", 0),
		}),
	})
	if err := ValidateServicePointer(inline); err == nil {
		t.Fatal("ValidateServicePointer inline IPC without start-command returned nil error")
	}

	inline.StartCommand = "go run ./cmd/inline-proxy"
	if err := ValidateServicePointer(inline); err != nil {
		t.Fatalf("ValidateServicePointer complete inline IPC service: %v", err)
	}
}

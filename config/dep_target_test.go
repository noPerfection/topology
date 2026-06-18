package config

import (
	"encoding/json"
	"testing"

	"github.com/noPerfection/protocol/message"
)

const (
	testAuthProxyLink        = "pkg:$?var=services[name:auth_proxy]"
	testAuthProxyHandlerLink = "pkg:$?var=services[name:auth_proxy].handlers[category:main]"
)

func TestDepTargetJSONLink(t *testing.T) {
	data, err := json.Marshal(testAuthProxyLink)
	if err != nil {
		t.Fatalf("Marshal test link: %v", err)
	}

	var target DepTarget
	if err := json.Unmarshal(data, &target); err != nil {
		t.Fatalf("Unmarshal link: %v", err)
	}
	if target.Link != testAuthProxyLink {
		t.Fatalf("Link = %q, want %q", target.Link, testAuthProxyLink)
	}
	if !target.IsLink() || target.IsInline() {
		t.Fatalf("IsLink = %v, IsInline = %v, want true/false", target.IsLink(), target.IsInline())
	}

	out, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal link: %v", err)
	}
	if string(out) != `"`+testAuthProxyLink+`"` {
		t.Fatalf("Marshal link = %s, want %q", string(out), testAuthProxyLink)
	}
}

func TestDepTargetJSONLinkWithHandler(t *testing.T) {
	data, err := json.Marshal(testAuthProxyHandlerLink)
	if err != nil {
		t.Fatalf("Marshal test link: %v", err)
	}

	var target DepTarget
	if err := json.Unmarshal(data, &target); err != nil {
		t.Fatalf("Unmarshal link: %v", err)
	}
	if target.Link != testAuthProxyHandlerLink {
		t.Fatalf("Link = %q, want %q", target.Link, testAuthProxyHandlerLink)
	}

	out, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal link: %v", err)
	}
	if string(out) != `"`+testAuthProxyHandlerLink+`"` {
		t.Fatalf("Marshal link = %s, want %q", string(out), testAuthProxyHandlerLink)
	}
}

func TestDepTargetMushroomLink(t *testing.T) {
	link := testAuthProxyLink
	var target DepTarget
	data, err := json.Marshal(link)
	if err != nil {
		t.Fatalf("Marshal test link: %v", err)
	}
	if err := json.Unmarshal(data, &target); err != nil {
		t.Fatalf("Unmarshal mushroom link: %v", err)
	}
	if target.Link != link {
		t.Fatalf("Link = %q, want %q", target.Link, link)
	}
}

func TestNewLinkTargetBuilder(t *testing.T) {
	serviceOnly := NewLinkTarget(testAuthProxyLink)
	if serviceOnly.Link != testAuthProxyLink {
		t.Fatalf("service link = %q, want %q", serviceOnly.Link, testAuthProxyLink)
	}

	target := NewLinkTarget(testAuthProxyHandlerLink)
	if target.Link != testAuthProxyHandlerLink {
		t.Fatalf("stored link = %q, want %q", target.Link, testAuthProxyHandlerLink)
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
	if target.Link != "" {
		t.Fatalf("Link = %q, want empty", target.Link)
	}
	if !target.IsInline() || target.IsLink() {
		t.Fatalf("IsInline = %v, IsLink = %v, want true/false", target.IsInline(), target.IsLink())
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
	if roundTrip.Service.Name != "inline_worker" {
		t.Fatalf("Service.Name = %q, want inline_worker", roundTrip.Service.Name)
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
			"outbounds": [{
				"type": "Independent",
				"name": "audit_sink",
				"handlers": [{
					"type": "Replier",
					"category": "main",
					"endpoint": {"id": "audit_sink_1", "port": 4302}
				}]
			}]
		}]
	}`)

	var target DepTarget
	if err := json.Unmarshal(data, &target); err != nil {
		t.Fatalf("Unmarshal inline proxy: %v", err)
	}
	if target.Link != "" {
		t.Fatalf("Link = %q, want empty", target.Link)
	}
	if target.Type != ProxyType {
		t.Fatalf("Service = %#v, want proxy", target.Service)
	}
	if target.Service.Name != "inline_audit" {
		t.Fatalf("Service = %#v, want inline_audit", target.Service)
	}
	proxyHandler, ok := target.Handlers[0].AsProxyHandler()
	if len(target.Handlers) != 1 || !ok || len(proxyHandler.Outbounds) != 1 {
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
	if err := ValidateDepTarget(DepTarget{Link: "a", Service: Service{Name: "b", Type: ProxyType}}); err == nil {
		t.Fatal("ValidateDepTarget link and service returned nil error")
	}
	if err := ValidateDepTarget(DepTarget{Service: Service{Name: "orphan", Type: IndependentType}}); err == nil {
		t.Fatal("ValidateDepTarget inline service without handlers returned nil error")
	}
	if err := ValidateDepTarget(NewLinkTarget(testAuthProxyLink)); err != nil {
		t.Fatalf("ValidateDepTarget link: %v", err)
	}
}

func TestDepTargetInlineIpcRequiresCompleteService(t *testing.T) {
	inline := NewInlineTarget(Service{
		Type: ProxyType,
		Name: "inline_proxy",
		Handlers: []Handler{ProxyHandler{
			IndependentHandler: IndependentHandler{
				Type:     SyncReplierType,
				Category: "main",
				Endpoint: message.NewEndpoint("tmp/inline_proxy", 0),
			},
		}},
	})
	if err := ValidateDepTarget(inline); err == nil {
		t.Fatal("ValidateDepTarget inline IPC without start-command returned nil error")
	}

	inline.StartCommand = "go run ./cmd/inline-proxy"
	if err := ValidateDepTarget(inline); err != nil {
		t.Fatalf("ValidateDepTarget complete inline IPC service: %v", err)
	}
}

func TestOutboundServiceAllowsMinimalInlineService(t *testing.T) {
	inline := Service{
		Type: ProxyType,
		Name: "default-name-proxy",
		Handlers: []Handler{ProxyHandler{
			IndependentHandler: IndependentHandler{
				Type:     SyncReplierType,
				Category: "main",
				Endpoint: message.NewEndpoint("tmp/default_name_proxy", 0),
			},
		}},
	}
	if err := ValidateOutboundService(inline); err != nil {
		t.Fatalf("ValidateOutboundService minimal inline IPC service: %v", err)
	}
}

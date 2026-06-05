package config

import (
	"encoding/json"
	"testing"

	"github.com/noPerfection/protocol/message"
)

func TestServiceJSONProxyHandlers(t *testing.T) {
	service := Service{
		Type: ProxyType,
		Name: "auth_proxy",
		Handlers: []HandlerVariant{
			NewProxyHandlerVariant(ProxyHandler{
				Handler: Handler{
					Type:     ReplierType,
					Category: "auth",
					Endpoint: message.NewEndpoint("auth_1", 4301),
				},
				Outbounds: []DepTarget{RefTarget("user_service")},
			}),
		},
	}

	data, err := json.Marshal(service)
	if err != nil {
		t.Fatalf("Marshal proxy service: %v", err)
	}

	var roundTrip Service
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("Unmarshal proxy service: %v", err)
	}
	if roundTrip.Name != "auth_proxy" {
		t.Fatalf("Name = %q, want auth_proxy", roundTrip.Name)
	}
	if len(roundTrip.Handlers) != 1 {
		t.Fatalf("Proxy handlers = %#v, want one outbound", roundTrip.Handlers)
	}
	proxyHandler := roundTrip.Handlers[0].AsProxyHandler()
	if len(proxyHandler.Outbounds) != 1 {
		t.Fatalf("Proxy handler outbounds = %#v, want one outbound", proxyHandler.Outbounds)
	}
	if proxyHandler.Outbounds[0].Ref != "user_service" {
		t.Fatalf("Outbound ref = %q, want user_service", proxyHandler.Outbounds[0].Ref)
	}
}

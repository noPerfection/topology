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
		Handlers: []Handler{
			ProxyHandler{
				IndependentHandler: IndependentHandler{
					Type:     ReplierType,
					Category: "auth",
					Endpoint: message.NewEndpoint("auth_1", 4301),
				},
				Outbounds: []Service{
					{
						Type: IndependentType,
						Name: "user_service",
						Handlers: []Handler{IndependentHandler{
							Type:     ReplierType,
							Category: "main",
							Endpoint: message.NewEndpoint("user_1", 4302),
						}},
					},
				},
			},
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
	proxyHandler, ok := roundTrip.Handlers[0].AsProxyHandler()
	if !ok {
		t.Fatal("handler is not a ProxyHandler")
	}
	if len(proxyHandler.Outbounds) != 1 {
		t.Fatalf("Proxy handler outbounds = %#v, want one outbound", proxyHandler.Outbounds)
	}
	if proxyHandler.Outbounds[0].Name != "user_service" {
		t.Fatalf("Outbound service = %q, want user_service", proxyHandler.Outbounds[0].Name)
	}
}

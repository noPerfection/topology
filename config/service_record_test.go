package config

import (
	"encoding/json"
	"testing"

	"github.com/noPerfection/protocol/message"
)

func TestServiceRecordJSONProxy(t *testing.T) {
	record := NewProxyRecord(Proxy{
		Service: Service{
			Type: ProxyType,
			Name: "auth_proxy",
		},
		Handlers: []ProxyHandler{
			{
				Handler: Handler{
					Type:     ReplierType,
					Category: "auth",
					Endpoint: message.NewEndpoint("auth_1", 4301),
				},
				Outbounds: []DepTarget{RefTarget("user_service")},
			},
		},
	})

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal proxy record: %v", err)
	}

	var roundTrip ServiceRecord
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("Unmarshal proxy record: %v", err)
	}
	if roundTrip.Proxy == nil {
		t.Fatal("round trip proxy is nil")
	}
	if roundTrip.Name != "auth_proxy" {
		t.Fatalf("Name = %q, want auth_proxy", roundTrip.Name)
	}
	if len(roundTrip.Proxy.Handlers) != 1 || len(roundTrip.Proxy.Handlers[0].Outbounds) != 1 {
		t.Fatalf("Proxy handlers = %#v, want one outbound", roundTrip.Proxy.Handlers)
	}
	if roundTrip.Proxy.Handlers[0].Outbounds[0].Ref != "user_service" {
		t.Fatalf("Outbound ref = %q, want user_service", roundTrip.Proxy.Handlers[0].Outbounds[0].Ref)
	}
}

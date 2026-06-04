package config

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ServiceRecord is a top-level service entry that stores either a Service or a
// Proxy. The embedded Service keeps common service fields available to topology
// code, while Proxy preserves proxy-specific handler data for JSON round trips.
type ServiceRecord struct {
	Service
	Proxy *Proxy `json:"-"`
}

func NewServiceRecord(service Service) ServiceRecord {
	return ServiceRecord{Service: service}
}

func NewProxyRecord(proxy Proxy) ServiceRecord {
	p := proxy
	return ServiceRecord{
		Service: serviceFromProxy(p),
		Proxy:   &p,
	}
}

func (r ServiceRecord) IsZero() bool {
	return r.Name == "" && r.Proxy == nil
}

func (r ServiceRecord) ValidateTypes() error {
	if r.Proxy != nil {
		return r.Proxy.ValidateTypes()
	}
	return r.Service.ValidateTypes()
}

func (r ServiceRecord) MarshalJSON() ([]byte, error) {
	if r.Proxy != nil {
		return json.Marshal(r.Proxy)
	}
	return json.Marshal(r.Service)
}

func (r *ServiceRecord) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("service record is empty")
	}

	var kind struct {
		Type Type `json:"type"`
	}
	if err := json.Unmarshal(trimmed, &kind); err != nil {
		return fmt.Errorf("service record type: %w", err)
	}
	if kind.Type == ProxyType {
		var proxy Proxy
		if err := json.Unmarshal(trimmed, &proxy); err != nil {
			return fmt.Errorf("service record proxy: %w", err)
		}
		*r = NewProxyRecord(proxy)
		return nil
	}

	var service Service
	if err := json.Unmarshal(trimmed, &service); err != nil {
		return fmt.Errorf("service record service: %w", err)
	}
	*r = NewServiceRecord(service)
	return nil
}

func serviceFromProxy(proxy Proxy) Service {
	service := proxy.Service
	service.Handlers = make([]Handler, len(proxy.Handlers))
	for i, handler := range proxy.Handlers {
		service.Handlers[i] = handler.Handler
	}
	return service
}

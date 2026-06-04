package config

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// DepTarget is either a service name reference, an inline Service definition,
// or an inline Proxy definition.
type DepTarget struct {
	Ref    string
	Inline *Service
	Proxy  *Proxy
}

// RefTarget returns a dependency on an existing service by name.
func RefTarget(name string) DepTarget {
	return DepTarget{Ref: name}
}

// InlineTarget returns a dependency on an inline service definition.
func InlineTarget(service Service) DepTarget {
	s := service
	return DepTarget{Inline: &s}
}

// ProxyTarget returns a dependency on an inline proxy definition.
func ProxyTarget(proxy Proxy) DepTarget {
	p := proxy
	return DepTarget{Proxy: &p}
}

// Name returns the service name for this target (ref or inline).
func (t DepTarget) Name() string {
	if t.Inline != nil {
		return t.Inline.Name
	}
	if t.Proxy != nil {
		return t.Proxy.Name
	}
	return t.Ref
}

// InlineService returns the service view for an inline service or proxy.
func (t DepTarget) InlineService() *Service {
	if t.Inline != nil {
		return t.Inline
	}
	if t.Proxy != nil {
		return t.Proxy.ServiceConfig()
	}
	return nil
}

// MarshalJSON encodes the target as a JSON string (ref), service object, or proxy object.
func (t DepTarget) MarshalJSON() ([]byte, error) {
	if err := ValidateDepTarget(t); err != nil {
		return nil, err
	}
	if t.Inline != nil {
		return json.Marshal(t.Inline)
	}
	if t.Proxy != nil {
		return json.Marshal(t.Proxy)
	}
	if t.Ref != "" {
		return json.Marshal(t.Ref)
	}
	return nil, fmt.Errorf("dep target is empty")
}

// UnmarshalJSON accepts a JSON string, service object, or proxy object.
func (t *DepTarget) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("dep target is empty")
	}

	if trimmed[0] == '"' {
		var name string
		if err := json.Unmarshal(trimmed, &name); err != nil {
			return fmt.Errorf("dep target ref: %w", err)
		}
		if name == "" {
			return fmt.Errorf("dep target ref is empty")
		}
		t.Ref = name
		t.Inline = nil
		t.Proxy = nil
		return nil
	}

	var kind struct {
		Type Type `json:"type"`
	}
	if err := json.Unmarshal(trimmed, &kind); err != nil {
		return fmt.Errorf("dep target inline object: %w", err)
	}
	if kind.Type == ProxyType {
		var proxy Proxy
		if err := json.Unmarshal(trimmed, &proxy); err != nil {
			return fmt.Errorf("dep target inline proxy: %w", err)
		}
		if proxy.Name == "" {
			return fmt.Errorf("inline proxy name is empty")
		}
		t.Proxy = &proxy
		t.Inline = nil
		t.Ref = ""
		return nil
	}

	var service Service
	if err := json.Unmarshal(trimmed, &service); err != nil {
		return fmt.Errorf("dep target inline service: %w", err)
	}
	if service.Name == "" {
		return fmt.Errorf("inline service name is empty")
	}
	t.Inline = &service
	t.Ref = ""
	t.Proxy = nil
	return nil
}

// ValidateDepTarget checks that the target is exactly one of ref, inline service, or inline proxy.
func ValidateDepTarget(t DepTarget) error {
	hasRef := t.Ref != ""
	hasInline := t.Inline != nil
	hasProxy := t.Proxy != nil
	count := 0
	for _, ok := range []bool{hasRef, hasInline, hasProxy} {
		if ok {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("dep target must set exactly one of ref, inline service, or inline proxy")
	}
	if hasRef {
		return nil
	}
	if hasInline {
		if err := t.Inline.ValidateTypes(); err != nil {
			return fmt.Errorf("inline service: %w", err)
		}
		return nil
	}
	if err := t.Proxy.ValidateTypes(); err != nil {
		return fmt.Errorf("inline proxy: %w", err)
	}
	return nil
}

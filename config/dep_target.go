package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// DepTarget is either a service reference path, an inline Service definition,
// or an inline Proxy definition.
type DepTarget struct {
	Ref    string
	Inline *Service
	Proxy  *Proxy
}

// RefTarget returns a dependency on an existing service by name.
// An optional handler category selects service/handler.
func RefTarget(service string, handlerCategory ...string) DepTarget {
	ref := service
	if len(handlerCategory) > 0 && handlerCategory[0] != "" {
		ref = service + "/" + handlerCategory[0]
	}
	return DepTarget{Ref: ref}
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

// RefPath returns the referenced service name and handler category.
// When the ref has no handler category, handlerCategory is empty.
func (t DepTarget) RefPath() (serviceName string, handlerCategory string) {
	if t.Ref == "" {
		return "", ""
	}
	service, handlerCategory, err := parseRefPath(t.Ref)
	if err != nil {
		return "", ""
	}
	return service, handlerCategory
}

// Name returns the service name for this target (ref or inline).
func (t DepTarget) Name() string {
	if t.Inline != nil {
		return t.Inline.Name
	}
	if t.Proxy != nil {
		return t.Proxy.Name
	}
	service, _ := t.RefPath()
	return service
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
		var ref string
		if err := json.Unmarshal(trimmed, &ref); err != nil {
			return fmt.Errorf("dep target ref: %w", err)
		}
		if _, _, err := parseRefPath(ref); err != nil {
			return err
		}
		t.Ref = ref
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

func parseRefPath(ref string) (service string, handlerCategory string, err error) {
	if ref == "" {
		return "", "", fmt.Errorf("dep target ref is empty")
	}
	if strings.HasSuffix(ref, "/") {
		return "", "", fmt.Errorf("dep target ref %q has empty handler category", ref)
	}
	if strings.Contains(ref, "//") {
		return "", "", fmt.Errorf("dep target ref %q has empty path segment", ref)
	}

	service, handlerCategory, ok := strings.Cut(ref, "/")
	if !ok {
		return ref, "", nil
	}
	if service == "" {
		return "", "", fmt.Errorf("dep target service name is empty")
	}
	if handlerCategory == "" {
		return "", "", fmt.Errorf("dep target handler category is empty")
	}
	return service, handlerCategory, nil
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
		if _, _, err := parseRefPath(t.Ref); err != nil {
			return err
		}
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

package config

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// DepTarget is either a service name reference or an inline Service definition.
type DepTarget struct {
	Ref    string
	Inline *Service
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

// Name returns the service name for this target (ref or inline).
func (t DepTarget) Name() string {
	if t.Inline != nil {
		return t.Inline.Name
	}
	return t.Ref
}

// MarshalJSON encodes the target as a JSON string (ref) or service object (inline).
func (t DepTarget) MarshalJSON() ([]byte, error) {
	if t.Inline != nil {
		return json.Marshal(t.Inline)
	}
	if t.Ref != "" {
		return json.Marshal(t.Ref)
	}
	return nil, fmt.Errorf("dep target is empty")
}

// UnmarshalJSON accepts either a JSON string or a service object.
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
	return nil
}

// ValidateDepTarget checks that the target is exactly one of ref or inline.
func ValidateDepTarget(t DepTarget) error {
	hasRef := t.Ref != ""
	hasInline := t.Inline != nil
	if hasRef && hasInline {
		return fmt.Errorf("dep target must not set both ref and inline")
	}
	if !hasRef && !hasInline {
		return fmt.Errorf("dep target must be a service ref or inline service")
	}
	if hasRef {
		return nil
	}
	if err := ValidateServiceType(t.Inline.Type); err != nil {
		return fmt.Errorf("inline service: %w", err)
	}
	for _, h := range t.Inline.Handlers {
		if err := ValidateHandlerType(h.Type); err != nil {
			return fmt.Errorf("inline service handler: %w", err)
		}
		if len(h.Category) == 0 {
			return fmt.Errorf("inline service handler category is empty")
		}
		for _, dep := range h.CommandDeps {
			if err := ValidateCommandDep(dep); err != nil {
				return fmt.Errorf("inline service ValidateCommandDep: %w", err)
			}
		}
	}
	return nil
}

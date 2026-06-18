package config

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// DepTarget is a dependency destination: a Mushroom link or an inline service.
type DepTarget struct {
	Link string
	Service
}

// NewLinkTarget returns a dependency on an existing resource via Mushroom link.
func NewLinkTarget(link string) DepTarget {
	return DepTarget{Link: link}
}

// NewInlineTarget returns a dependency on an inline service definition.
func NewInlineTarget(service Service) DepTarget {
	return DepTarget{Service: service}
}

// IsLink reports whether the target is a Mushroom link.
func (t DepTarget) IsLink() bool {
	return t.Link != ""
}

// IsInline reports whether the target is an inline service definition.
func (t DepTarget) IsInline() bool {
	return t.Service.Name != ""
}

// MarshalJSON encodes the target as a JSON string (link) or service object.
func (t DepTarget) MarshalJSON() ([]byte, error) {
	if t.IsLink() == t.IsInline() {
		return nil, fmt.Errorf("dep target must set exactly one of link or inline service")
	}
	if t.IsLink() {
		return json.Marshal(t.Link)
	}
	return json.Marshal(t.Service)
}

// UnmarshalJSON accepts a JSON string or service object.
func (t *DepTarget) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("dep target is empty")
	}

	if trimmed[0] == '"' {
		var link string
		if err := json.Unmarshal(trimmed, &link); err != nil {
			return fmt.Errorf("dep target link: %w", err)
		}
		t.Link = link
		t.Service = Service{}
		return nil
	}

	var service Service
	if err := json.Unmarshal(trimmed, &service); err != nil {
		return fmt.Errorf("dep target service: %w", err)
	}
	t.Link = ""
	t.Service = service
	return nil
}

// ValidateDepTarget checks that the target is exactly one of link or inline service.
func ValidateDepTarget(t DepTarget) error {
	if t.IsLink() == t.IsInline() {
		return fmt.Errorf("dep target must set exactly one of link or inline service")
	}
	if t.IsLink() {
		return nil
	}
	if err := ValidateService(t.Service); err != nil {
		return fmt.Errorf("service: %w", err)
	}
	// Inline service must have at least one handler.
	if len(t.Service.Handlers) == 0 {
		return fmt.Errorf("inline service must have at least one handler")
	}
	return nil
}

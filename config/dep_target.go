package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// DepTarget is either a service reference path or a service record.
type DepTarget struct {
	Ref string
	ServiceRecord
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

// ServiceTarget returns a dependency on an inline service definition.
func ServiceTarget(service Service) DepTarget {
	return DepTarget{ServiceRecord: NewServiceRecord(service)}
}

// ProxyTarget returns a dependency on an inline proxy definition.
func ProxyTarget(proxy Proxy) DepTarget {
	return DepTarget{ServiceRecord: NewProxyRecord(proxy)}
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

// Name returns the service name for this target (ref or record).
func (t DepTarget) Name() string {
	if !t.ServiceRecord.IsZero() {
		return t.ServiceRecord.Name
	}
	service, _ := t.RefPath()
	return service
}

// MarshalJSON encodes the target as a JSON string (ref), service object, or proxy object.
func (t DepTarget) MarshalJSON() ([]byte, error) {
	if err := ValidateDepTarget(t); err != nil {
		return nil, err
	}
	if !t.ServiceRecord.IsZero() {
		return json.Marshal(t.ServiceRecord)
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
		t.ServiceRecord = ServiceRecord{}
		return nil
	}

	var record ServiceRecord
	if err := json.Unmarshal(trimmed, &record); err != nil {
		return fmt.Errorf("dep target service record: %w", err)
	}
	t.Ref = ""
	t.ServiceRecord = record
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

// ValidateDepTarget checks that the target is exactly one of ref or service record.
func ValidateDepTarget(t DepTarget) error {
	hasRef := t.Ref != ""
	hasRecord := !t.ServiceRecord.IsZero()
	count := 0
	for _, ok := range []bool{hasRef, hasRecord} {
		if ok {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("dep target must set exactly one of ref or service record")
	}
	if hasRef {
		if _, _, err := parseRefPath(t.Ref); err != nil {
			return err
		}
		return nil
	}
	if err := t.ServiceRecord.ValidateTypes(); err != nil {
		return fmt.Errorf("service record: %w", err)
	}
	return nil
}

package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// ServicePointer is either a service reference path or an inline service definition.
type ServicePointer struct {
	Ref string
	Service
}

// RefTarget returns a dependency on an existing service by name.
// An optional handler category selects service/handler.
func RefTarget(service string, handlerCategory ...string) ServicePointer {
	ref := service
	if len(handlerCategory) > 0 && handlerCategory[0] != "" {
		ref = service + "/" + handlerCategory[0]
	}
	return ServicePointer{Ref: ref}
}

// ServiceTarget returns a dependency on an inline service definition.
func ServiceTarget(service Service) ServicePointer {
	return ServicePointer{Service: service}
}

// RefPath returns the referenced service name and handler category.
// When the ref has no handler category, handlerCategory is empty.
func (t ServicePointer) RefPath() (serviceName string, handlerCategory string) {
	if t.Ref == "" {
		return "", ""
	}
	service, handlerCategory, err := parseRefPath(t.Ref)
	if err != nil {
		return "", ""
	}
	return service, handlerCategory
}

// Name returns the service name for this target (ref or inline service).
func (t ServicePointer) Name() string {
	if !t.Service.IsZero() {
		return t.Service.Name
	}
	service, _ := t.RefPath()
	return service
}

// MarshalJSON encodes the target as a JSON string (ref) or service object.
func (t ServicePointer) MarshalJSON() ([]byte, error) {
	if err := ValidateServicePointer(t); err != nil {
		return nil, err
	}
	if !t.Service.IsZero() {
		return json.Marshal(t.Service)
	}
	if t.Ref != "" {
		return json.Marshal(t.Ref)
	}
	return nil, fmt.Errorf("dep target is empty")
}

// UnmarshalJSON accepts a JSON string or service object.
func (t *ServicePointer) UnmarshalJSON(data []byte) error {
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
		t.Service = Service{}
		return nil
	}

	var service Service
	if err := json.Unmarshal(trimmed, &service); err != nil {
		return fmt.Errorf("dep target service: %w", err)
	}
	t.Ref = ""
	t.Service = service
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

// ValidateServicePointer checks that the target is exactly one of ref or inline service.
func ValidateServicePointer(t ServicePointer) error {
	hasRef := t.Ref != ""
	hasRecord := !t.Service.IsZero()
	count := 0
	for _, ok := range []bool{hasRef, hasRecord} {
		if ok {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("dep target must set exactly one of ref or inline service")
	}
	if hasRef {
		if _, _, err := parseRefPath(t.Ref); err != nil {
			return err
		}
		return nil
	}
	if err := ValidateService(t.Service); err != nil {
		return fmt.Errorf("service: %w", err)
	}
	return nil
}

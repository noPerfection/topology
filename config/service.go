package config

import (
	"fmt"
	"slices"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
)

type DepService struct {
	// For command deps its command, for handler deps its handler catego
	Name       string      `json:"name"`
	Proxies    []DepTarget `json:"proxies,omitempty"`
	Extensions []DepTarget `json:"extensions,omitempty"`
}

type Handler struct {
	Type        HandlerType      `json:"type"`
	Category    string           `json:"category"`
	Endpoint    message.Endpoint `json:"endpoint"`
	CommandDeps []DepService     `json:"command-deps,omitempty"`
}

// Service type defined in the config.
//
// Fields
//   - Type is the type of service. For example, ProxyType, IndependentType or ExtensionType
//   - Name of the service
//   - Handlers that are listed in the service
//   - Parameters optional service-local metadata (not validated)
type Service struct {
	Type         Type              `json:"type"`
	Name         string            `json:"name"`
	ModuleUrl    string            `json:"module-url,omitempty"`
	StartCommand string            `json:"start-command,omitempty"`
	HandlerDeps  []DepService      `json:"handler-deps,omitempty"`
	Handlers     []Handler         `json:"handlers"`
	Parameters   datatype.KeyValue `json:"parameters,omitempty"`
}

// New generates a service configuration.
func New(name string, serviceType Type) *Service {
	return &Service{
		Type:     serviceType,
		Name:     name,
		Handlers: make([]Handler, 0),
	}
}

// ValidateService validates the service metadata and endpoint bootstrap settings.
func ValidateService(service Service) error {
	if len(service.Name) == 0 {
		return fmt.Errorf("service name is empty")
	}
	if err := ValidateServiceType(service.Type); err != nil {
		return fmt.Errorf("identity.ValidateServiceType: %v", err)
	}

	needsModuleURL := false
	needsStartCommand := false
	for i, dep := range service.HandlerDeps {
		if err := ValidateDepService(dep); err != nil {
			return fmt.Errorf("ValidateHandlerDep[%d]: %v", i, err)
		}
	}

	for i, h := range service.Handlers {
		if err := ValidateHandlerType(h.Type); err != nil {
			return fmt.Errorf("ValidateHandlerType[%d]: %v", i, err)
		}
		if len(h.Category) == 0 {
			return fmt.Errorf("handler[%d] category is empty", i)
		}
		if len(h.Endpoint.Id) == 0 && !h.Endpoint.IsRemote() {
			return fmt.Errorf("handler[%d] '%s' endpoint id is empty", i, h.Category)
		}

		if h.Endpoint.IsInproc() {
			needsModuleURL = true
		}
		if h.Endpoint.IsIpc() {
			needsStartCommand = true
		}

		for _, dep := range h.CommandDeps {
			if err := ValidateDepService(dep); err != nil {
				return fmt.Errorf("ValidateCommandDepService[%d]: %v", i, err)
			}
		}
	}

	if needsModuleURL && len(service.ModuleUrl) == 0 {
		return fmt.Errorf("service('%s') has inproc endpoint and requires module-url", service.Name)
	}
	if needsStartCommand && len(service.StartCommand) == 0 {
		return fmt.Errorf("service('%s') has ipc endpoint and requires start-command", service.Name)
	}

	return nil
}

// ValidateTypes validates the parameters of the service.
func (s *Service) ValidateTypes() error {
	if s == nil {
		return fmt.Errorf("service struct is nil")
	}
	return ValidateService(*s)
}

// HandlerByCategory returns the handler config by the handler category.
// If the handler doesn't exist, then it returns an error.
func (s *Service) HandlerByCategory(category string) (Handler, error) {
	if len(category) == 0 {
		return Handler{}, fmt.Errorf("category argument is empty")
	}

	i := slices.IndexFunc(s.Handlers, func(e Handler) bool {
		return e.Category == category
	})
	if i == -1 {
		return Handler{}, fmt.Errorf("handler of '%s' category not found", category)
	}

	return s.Handlers[i], nil
}

// GetHandler returns a handler by its endpoint.
func (s *Service) GetHandler(endpoint message.Endpoint) (Handler, error) {
	if s == nil {
		return Handler{}, fmt.Errorf("service struct is nil")
	}
	if len(endpoint.Id) == 0 && !endpoint.IsRemote() {
		return Handler{}, fmt.Errorf("endpoint id argument is empty")
	}

	i := slices.IndexFunc(s.Handlers, func(h Handler) bool {
		return h.Endpoint.Id == endpoint.Id && h.Endpoint.Port == endpoint.Port
	})
	if i == -1 {
		return Handler{}, fmt.Errorf("handler with endpoint '%s:%d' not found", endpoint.Id, endpoint.Port)
	}

	return s.Handlers[i], nil
}

// SetHandler adds a new handler.
// If the handler with the same endpoint is identical, it will over-write that handler.
// Otherwise, it will add a new handler.
func (s *Service) SetHandler(handler Handler, overwriteByCategory ...bool) {
	if s == nil {
		return
	}

	if len(s.Handlers) == 0 {
		s.Handlers = []Handler{handler}
		return
	}

	var i int
	if len(overwriteByCategory) > 0 && overwriteByCategory[0] {
		i = slices.IndexFunc(s.Handlers, func(h Handler) bool {
			return h.Category == handler.Category
		})
	} else {
		i = slices.IndexFunc(s.Handlers, func(h Handler) bool {
			return h.Endpoint == handler.Endpoint
		})
	}

	if i == -1 {
		s.Handlers = append(s.Handlers, handler)
		return
	}

	s.Handlers[i] = handler
}

// RemoveHandler removes a handler by its endpoint.
func (s *Service) RemoveHandler(endpoint message.Endpoint) error {
	if s == nil {
		return fmt.Errorf("service struct is nil")
	}
	if len(endpoint.Id) == 0 && !endpoint.IsRemote() {
		return fmt.Errorf("endpoint id argument is empty")
	}

	i := slices.IndexFunc(s.Handlers, func(h Handler) bool {
		return h.Endpoint.Id == endpoint.Id && h.Endpoint.Port == endpoint.Port
	})
	if i == -1 {
		return fmt.Errorf("handler with endpoint '%s:%d' not found", endpoint.Id, endpoint.Port)
	}

	s.Handlers = slices.Delete(s.Handlers, i, i+1)
	return nil
}

// ValidateDepService checks that a dependency declares a name and routing targets.
func ValidateDepService(dep DepService) error {
	if len(dep.Name) == 0 {
		return fmt.Errorf("name argument is empty")
	}
	if len(dep.Proxies) == 0 && len(dep.Extensions) == 0 {
		return fmt.Errorf("dep service('%s') must declare proxies or extensions", dep.Name)
	}

	for i, target := range dep.Proxies {
		if err := ValidateDepTarget(target); err != nil {
			return fmt.Errorf("proxies[%d]: %w", i, err)
		}
	}
	for i, target := range dep.Extensions {
		if err := ValidateDepTarget(target); err != nil {
			return fmt.Errorf("extensions[%d]: %w", i, err)
		}
	}

	return nil
}

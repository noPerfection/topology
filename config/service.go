package config

import (
	"bytes"
	"encoding/json"
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

type ProxyHandler struct {
	Handler
	Routes    []string    `json:"routes,omitempty"` // whitelist routes
	Outbounds []DepTarget `json:"outbounds"`
}

type HandlerVariant struct {
	Handler      *Handler      `json:"-"`
	ProxyHandler *ProxyHandler `json:"-"`
}

func NewHandlerVariant(handler Handler) HandlerVariant {
	h := handler
	return HandlerVariant{Handler: &h}
}

func NewHandlerVariants(handlers ...Handler) []HandlerVariant {
	variants := make([]HandlerVariant, len(handlers))
	for i, handler := range handlers {
		variants[i] = NewHandlerVariant(handler)
	}
	return variants
}

func NewProxyHandlerVariant(handler ProxyHandler) HandlerVariant {
	h := handler
	return HandlerVariant{ProxyHandler: &h}
}

func (h HandlerVariant) AsHandler() Handler {
	if h.ProxyHandler != nil {
		return h.ProxyHandler.Handler
	}
	if h.Handler != nil {
		return *h.Handler
	}
	return Handler{}
}

func (h HandlerVariant) AsProxyHandler() ProxyHandler {
	if h.ProxyHandler != nil {
		return *h.ProxyHandler
	}
	return ProxyHandler{Handler: h.AsHandler()}
}

func (h HandlerVariant) MarshalJSON() ([]byte, error) {
	if h.ProxyHandler != nil {
		return json.Marshal(h.ProxyHandler)
	}
	if h.Handler != nil {
		return json.Marshal(h.Handler)
	}
	return nil, fmt.Errorf("handler variant is empty")
}

func (h *HandlerVariant) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("handler variant is empty")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return fmt.Errorf("handler variant object: %w", err)
	}
	if _, ok := raw["outbounds"]; ok {
		var handler ProxyHandler
		if err := json.Unmarshal(trimmed, &handler); err != nil {
			return fmt.Errorf("proxy handler: %w", err)
		}
		*h = NewProxyHandlerVariant(handler)
		return nil
	}
	if _, ok := raw["routes"]; ok {
		var handler ProxyHandler
		if err := json.Unmarshal(trimmed, &handler); err != nil {
			return fmt.Errorf("proxy handler: %w", err)
		}
		*h = NewProxyHandlerVariant(handler)
		return nil
	}

	var handler Handler
	if err := json.Unmarshal(trimmed, &handler); err != nil {
		return fmt.Errorf("handler: %w", err)
	}
	*h = NewHandlerVariant(handler)
	return nil
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
	Handlers     []HandlerVariant  `json:"handlers"`
	Parameters   datatype.KeyValue `json:"parameters,omitempty"`
}

func (s Service) IsZero() bool {
	return s.Name == ""
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
		handler := h.AsHandler()
		if err := ValidateHandlerType(handler.Type); err != nil {
			return fmt.Errorf("ValidateHandlerType[%d]: %v", i, err)
		}
		if len(handler.Category) == 0 {
			return fmt.Errorf("handler[%d] category is empty", i)
		}
		if len(handler.Endpoint.Id) == 0 && !handler.Endpoint.IsRemote() {
			return fmt.Errorf("handler[%d] '%s' endpoint id is empty", i, handler.Category)
		}

		if handler.Endpoint.IsInproc() {
			needsModuleURL = true
		}
		if handler.Endpoint.IsIpc() {
			needsStartCommand = true
		}

		for _, dep := range handler.CommandDeps {
			if err := ValidateDepService(dep); err != nil {
				return fmt.Errorf("ValidateCommandDepService[%d]: %v", i, err)
			}
		}
		if service.Type == ProxyType {
			proxyHandler := h.AsProxyHandler()
			for j, target := range proxyHandler.Outbounds {
				if err := ValidateDepTarget(target); err != nil {
					return fmt.Errorf("handler[%d] outbounds[%d]: %w", i, j, err)
				}
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

// HandlerByCategory returns the handler config by the handler category.
// If the handler doesn't exist, then it returns an error.
func (s *Service) HandlerByCategory(category string) (HandlerVariant, error) {
	if len(category) == 0 {
		return HandlerVariant{}, fmt.Errorf("category argument is empty")
	}

	i := slices.IndexFunc(s.Handlers, func(e HandlerVariant) bool {
		return e.AsHandler().Category == category
	})
	if i == -1 {
		return HandlerVariant{}, fmt.Errorf("handler of '%s' category not found", category)
	}

	return s.Handlers[i], nil
}

// GetHandler returns a handler by its endpoint.
func (s *Service) GetHandler(endpoint message.Endpoint) (HandlerVariant, error) {
	if s == nil {
		return HandlerVariant{}, fmt.Errorf("service struct is nil")
	}
	if len(endpoint.Id) == 0 && !endpoint.IsRemote() {
		return HandlerVariant{}, fmt.Errorf("endpoint id argument is empty")
	}

	i := slices.IndexFunc(s.Handlers, func(h HandlerVariant) bool {
		handler := h.AsHandler()
		return handler.Endpoint.Id == endpoint.Id && handler.Endpoint.Port == endpoint.Port
	})
	if i == -1 {
		return HandlerVariant{}, fmt.Errorf("handler with endpoint '%s:%d' not found", endpoint.Id, endpoint.Port)
	}

	return s.Handlers[i], nil
}

// SetHandler adds a new handler.
// If the handler with the same endpoint is identical, it will over-write that handler.
// Otherwise, it will add a new handler.
func (s *Service) SetHandler(handler HandlerVariant, overwriteByCategory ...bool) {
	if s == nil {
		return
	}
	baseHandler := handler.AsHandler()

	if len(s.Handlers) == 0 {
		s.Handlers = []HandlerVariant{handler}
		return
	}

	var i int
	if len(overwriteByCategory) > 0 && overwriteByCategory[0] {
		i = slices.IndexFunc(s.Handlers, func(h HandlerVariant) bool {
			return h.AsHandler().Category == baseHandler.Category
		})
	} else {
		i = slices.IndexFunc(s.Handlers, func(h HandlerVariant) bool {
			return h.AsHandler().Endpoint == baseHandler.Endpoint
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

	i := slices.IndexFunc(s.Handlers, func(h HandlerVariant) bool {
		handler := h.AsHandler()
		return handler.Endpoint.Id == endpoint.Id && handler.Endpoint.Port == endpoint.Port
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

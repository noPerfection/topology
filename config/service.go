package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
)

const (
	// All services must have a manager handler. Identified by this category.
	ServiceManagerCategory = "manager"
	// For proxy or extension services, use "inproc-handlers" to list handler categories that should be treated as inproc.
	// Its in the service parameters: parameters.inproc-handlers: [list of handler categories]
	InprocHandlersParameter = "inproc-handlers"
)

// Command Deps or Service deps per handler of service.
// Use it to pipe other services
type DepService struct {
	// For command deps its command, for handler deps its handler catego
	Name       string           `json:"name"`
	Proxies    []ServicePointer `json:"proxies,omitempty"`
	Extensions []ServicePointer `json:"extensions,omitempty"`
}

type IndependentHandler struct {
	Type        HandlerType      `json:"type"`
	Category    string           `json:"category"`
	Endpoint    message.Endpoint `json:"endpoint"`
	CommandDeps []DepService     `json:"command-deps,omitempty"`
}

type ProxyHandler struct {
	IndependentHandler
	Routes    []string          `json:"routes,omitempty"` // whitelist routes
	Outbounds []Service         `json:"outbounds"`
	Forward   map[string]string `json:"forward,omitempty"` // command route => outbound ref
}

type Handler interface {
	isHandler()
	AsIndependentHandler() (IndependentHandler, bool)
	AsProxyHandler() (ProxyHandler, bool)
}

func (h IndependentHandler) isHandler() {}

func (h IndependentHandler) AsIndependentHandler() (IndependentHandler, bool) {
	return h, true
}

func (h IndependentHandler) AsProxyHandler() (ProxyHandler, bool) {
	return ProxyHandler{}, false
}

func (h ProxyHandler) isHandler() {}

func (h ProxyHandler) AsIndependentHandler() (IndependentHandler, bool) {
	return h.IndependentHandler, true
}

func (h ProxyHandler) AsProxyHandler() (ProxyHandler, bool) {
	return h, true
}

func unmarshalHandler(data []byte) (Handler, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, fmt.Errorf("handler is empty")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return nil, fmt.Errorf("handler object: %w", err)
	}
	if _, ok := raw["outbounds"]; ok {
		var handler ProxyHandler
		if err := json.Unmarshal(trimmed, &handler); err != nil {
			return nil, fmt.Errorf("proxy handler: %w", err)
		}
		return handler, nil
	}
	if _, ok := raw["routes"]; ok {
		var handler ProxyHandler
		if err := json.Unmarshal(trimmed, &handler); err != nil {
			return nil, fmt.Errorf("proxy handler: %w", err)
		}
		return handler, nil
	}
	if _, ok := raw["forward"]; ok {
		var handler ProxyHandler
		if err := json.Unmarshal(trimmed, &handler); err != nil {
			return nil, fmt.Errorf("proxy handler: %w", err)
		}
		return handler, nil
	}

	var handler IndependentHandler
	if err := json.Unmarshal(trimmed, &handler); err != nil {
		return nil, fmt.Errorf("handler: %w", err)
	}
	return handler, nil
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

func (s *Service) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type         Type              `json:"type"`
		Name         string            `json:"name"`
		ModuleUrl    string            `json:"module-url,omitempty"`
		StartCommand string            `json:"start-command,omitempty"`
		HandlerDeps  []DepService      `json:"handler-deps,omitempty"`
		Handlers     []json.RawMessage `json:"handlers"`
		Parameters   datatype.KeyValue `json:"parameters,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	handlers := make([]Handler, len(raw.Handlers))
	for i, rawHandler := range raw.Handlers {
		handler, err := unmarshalHandler(rawHandler)
		if err != nil {
			return fmt.Errorf("handlers[%d]: %w", i, err)
		}
		handlers[i] = handler
	}

	*s = Service{
		Type:         raw.Type,
		Name:         raw.Name,
		ModuleUrl:    raw.ModuleUrl,
		StartCommand: raw.StartCommand,
		HandlerDeps:  raw.HandlerDeps,
		Handlers:     handlers,
		Parameters:   raw.Parameters,
	}
	return nil
}

func (s Service) IsZero() bool {
	return s.Name == ""
}

// If service is not Inproc, and any handler is IPC except the ServiceManagerCategory,
// then the service is IPC.
func (s Service) IsIpc() bool {
	if s.IsInproc() {
		return false
	}
	for _, variant := range s.Handlers {
		if variant == nil {
			continue
		}
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		if handler.Category == ServiceManagerCategory {
			continue
		}
		if handler.Endpoint.IsIpc() {
			return true
		}
	}
	return false
}

// If any handler except ServiceManagerCategory is inproc, then the service is inproc.
// For proxy or extension type, the service is inproc if the handler category is listed in parameters.inproc-handlers.
func (s Service) IsInproc() bool {
	for _, variant := range s.Handlers {
		if variant == nil {
			continue
		}
		handler, ok := variant.AsIndependentHandler()
		if !ok {
			continue
		}
		if handler.Category == ServiceManagerCategory {
			continue
		}

		if s.Type == ProxyType || s.Type == ExtensionType {
			if serviceParameterHasInprocHandler(s, handler.Category) {
				return true
			}
		}

		if handler.Endpoint.IsInproc() {
			return true
		}
	}
	return false
}

// IsInprocHandler reports whether the handler with the given category should be
// treated as in-process for the service. For Proxy and Extension services, a
// handler listed in parameters.inproc-handlers is treated as inproc even when
// its endpoint is IPC or TCP. Otherwise the handler endpoint is used.
func (s Service) IsInprocHandler(category string) (bool, error) {
	handlerVariant, err := s.HandlerByCategory(category)
	if err != nil {
		return false, err
	}
	handler, ok := handlerVariant.AsIndependentHandler()
	if !ok {
		return false, fmt.Errorf("handler of '%s' category is not an independent handler", category)
	}
	if s.Type == ProxyType || s.Type == ExtensionType {
		if serviceParameterHasInprocHandler(s, category) {
			return true, nil
		}
	}
	return handler.Endpoint.IsInproc(), nil
}

func serviceParameterHasInprocHandler(service Service, category string) bool {
	if service.Parameters == nil || category == "" {
		return false
	}
	raw, exists := service.Parameters[InprocHandlersParameter]
	if !exists {
		return false
	}
	switch categories := raw.(type) {
	case []string:
		return slices.Contains(categories, category)
	case []interface{}:
		for _, item := range categories {
			if name, ok := item.(string); ok && name == category {
				return true
			}
		}
	}
	return false
}

// ValidateOutboundService validates a minimal inline service used as a proxy outbound.
// Bootstrap fields such as module-url and start-command are not required.
func ValidateOutboundService(service Service) error {
	if len(service.Name) == 0 {
		return fmt.Errorf("service name is empty")
	}
	if err := ValidateServiceType(service.Type); err != nil {
		return fmt.Errorf("identity.ValidateServiceType: %v", err)
	}
	if len(service.Handlers) == 0 {
		return fmt.Errorf("service %q must have at least one handler", service.Name)
	}

	for i, h := range service.Handlers {
		if h == nil {
			return fmt.Errorf("handler[%d] is empty", i)
		}
		handler, ok := h.AsIndependentHandler()
		if !ok {
			return fmt.Errorf("handler[%d] is not an independent handler", i)
		}
		if err := ValidateHandlerType(handler.Type); err != nil {
			return fmt.Errorf("ValidateHandlerType[%d]: %v", i, err)
		}
		if len(handler.Category) == 0 {
			return fmt.Errorf("handler[%d] category is empty", i)
		}
		if len(handler.Endpoint.Id) == 0 && !handler.Endpoint.IsRemote() {
			return fmt.Errorf("handler[%d] '%s' endpoint id is empty", i, handler.Category)
		}
	}

	return nil
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
		if h == nil {
			return fmt.Errorf("handler[%d] is empty", i)
		}
		handler, ok := h.AsIndependentHandler()
		if !ok {
			return fmt.Errorf("handler[%d] is not an independent handler", i)
		}
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
			proxyHandler, ok := h.AsProxyHandler()
			if !ok {
				return fmt.Errorf("handler[%d] must be a proxy handler", i)
			}
			for j, target := range proxyHandler.Outbounds {
				if err := ValidateOutboundService(target); err != nil {
					return fmt.Errorf("handler[%d] outbounds[%d]: %w", i, j, err)
				}
			}
			if err := ValidateProxyForwards(proxyHandler); err != nil {
				return fmt.Errorf("handler[%d] forward: %w", i, err)
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

func ValidateProxyForwards(proxyHandler ProxyHandler) error {
	for route, outboundRef := range proxyHandler.Forward {
		if !slices.Contains(proxyHandler.Routes, route) {
			return fmt.Errorf("route %q is not listed in routes", route)
		}
		if !proxyHandlerHasOutboundRef(proxyHandler, outboundRef) {
			return fmt.Errorf("outbound %q is not listed in outbounds", outboundRef)
		}
	}
	return nil
}

func proxyHandlerHasOutboundRef(proxyHandler ProxyHandler, ref string) bool {
	serviceName, handlerCategory, err := parseRefPath(ref)
	if err != nil || serviceName == "" {
		return false
	}
	if handlerCategory == "" {
		handlerCategory = "main"
	}

	for _, outbound := range proxyHandler.Outbounds {
		if outbound.Name != serviceName {
			continue
		}
		if _, err := outbound.HandlerByCategory(handlerCategory); err == nil {
			return true
		}
	}
	return false
}

// HandlerByCategory returns the handler config by the handler category.
// If the handler doesn't exist, then it returns an error.
func (s *Service) HandlerByCategory(category string) (Handler, error) {
	if len(category) == 0 {
		return nil, fmt.Errorf("category argument is empty")
	}

	i := slices.IndexFunc(s.Handlers, func(e Handler) bool {
		if e == nil {
			return false
		}
		handler, ok := e.AsIndependentHandler()
		return ok && handler.Category == category
	})
	if i == -1 {
		return nil, fmt.Errorf("handler of '%s' category not found", category)
	}

	return s.Handlers[i], nil
}

// GetHandler returns a handler by its endpoint.
func (s *Service) GetHandler(endpoint message.Endpoint) (Handler, error) {
	if s == nil {
		return nil, fmt.Errorf("service struct is nil")
	}
	if len(endpoint.Id) == 0 && !endpoint.IsRemote() {
		return nil, fmt.Errorf("endpoint id argument is empty")
	}

	i := slices.IndexFunc(s.Handlers, func(h Handler) bool {
		if h == nil {
			return false
		}
		handler, ok := h.AsIndependentHandler()
		return ok && handler.Endpoint.Id == endpoint.Id && handler.Endpoint.Port == endpoint.Port
	})
	if i == -1 {
		return nil, fmt.Errorf("handler with endpoint '%s:%d' not found", endpoint.Id, endpoint.Port)
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
	if handler == nil {
		return
	}
	baseHandler, ok := handler.AsIndependentHandler()
	if !ok {
		return
	}

	if len(s.Handlers) == 0 {
		s.Handlers = []Handler{handler}
		return
	}

	var i int
	if len(overwriteByCategory) > 0 && overwriteByCategory[0] {
		i = slices.IndexFunc(s.Handlers, func(h Handler) bool {
			if h == nil {
				return false
			}
			handler, ok := h.AsIndependentHandler()
			return ok && handler.Category == baseHandler.Category
		})
	} else {
		i = slices.IndexFunc(s.Handlers, func(h Handler) bool {
			if h == nil {
				return false
			}
			handler, ok := h.AsIndependentHandler()
			return ok && handler.Endpoint == baseHandler.Endpoint
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
		if h == nil {
			return false
		}
		handler, ok := h.AsIndependentHandler()
		return ok && handler.Endpoint.Id == endpoint.Id && handler.Endpoint.Port == endpoint.Port
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
		if err := ValidateServicePointer(target); err != nil {
			return fmt.Errorf("proxies[%d]: %w", i, err)
		}
	}
	for i, target := range dep.Extensions {
		if err := ValidateServicePointer(target); err != nil {
			return fmt.Errorf("extensions[%d]: %w", i, err)
		}
	}

	return nil
}

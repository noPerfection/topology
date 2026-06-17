// Package config defines the noPerfection application configuration.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ahmetson/mushroom/substrates/json_substrate"
)

// NoPerfection is the configuration of the entire application.
// Consists the supported services.
type NoPerfection struct {
	mycelium *json_substrate.Mycelium
	filePath string
}

// normalizeServiceURL maps a service lookup argument to a dereference Mushroom URL.
// Mushroom URLs are returned unchanged; a plain symbol is expanded to a root name filter.
//
//	symbol:      "auth_proxy"  →  "pkg:$?*var=services[name:auth_proxy]"
//	mushroomURL: "pkg:$?*var=services[name:auth_proxy]"  →  unchanged
func normalizeServiceURL(mushroomURL string) string {
	if strings.HasPrefix(mushroomURL, "pkg:") || strings.HasPrefix(mushroomURL, "*pkg:") {
		return mushroomURL
	}
	return fmt.Sprintf("pkg:$?*var=services[name:%s]", mushroomURL)
}

func (a *NoPerfection) queryMycelium(mushroomURL string) (any, error) {
	if a == nil {
		return nil, fmt.Errorf("app struct is nil")
	}
	if a.mycelium == nil {
		return nil, fmt.Errorf("topology mycelium not set, call config.Load()")
	}

	spored, err := a.mycelium.Spore(mushroomURL)
	if err != nil {
		return nil, fmt.Errorf("mycelium.Spore(%q): %w", mushroomURL, err)
	}

	fruited, err := a.mycelium.Fruit(spored)
	if err != nil {
		return nil, fmt.Errorf("mycelium.Fruit: %w", err)
	}

	return fruited, nil
}

func decodeService(value any) (Service, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return Service{}, fmt.Errorf("value is not a service: %w", err)
	}

	var service Service
	if err := json.Unmarshal(data, &service); err != nil {
		return Service{}, fmt.Errorf("value is not a service: %w", err)
	}
	if service.Name == "" {
		return Service{}, fmt.Errorf("value is not a service: missing name")
	}

	return service, nil
}

func decodeServices(value any) ([]Service, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("mushroom url is fetching wrong data: expected array, got %T", value)
	}

	services := make([]Service, 0, len(items))
	for _, item := range items {
		service, err := decodeService(item)
		if err != nil {
			return nil, fmt.Errorf("mushroom url is fetching wrong data: %w", err)
		}
		services = append(services, service)
	}

	return services, nil
}

func encodeServiceMap(record Service) (map[string]any, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal service: %w", err)
	}

	var serviceMap map[string]any
	if err := json.Unmarshal(data, &serviceMap); err != nil {
		return nil, fmt.Errorf("json.Unmarshal service map: %w", err)
	}

	return serviceMap, nil
}

func unwrapServiceValue(value any) (any, error) {
	items, ok := value.([]any)
	if !ok {
		return value, nil
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("service not found")
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("multiple services matched")
	}
	return items[0], nil
}

func serviceExists(services []Service, name string) bool {
	for _, service := range services {
		if service.Name == name {
			return true
		}
	}
	return false
}

// Load loads an app configuration from a JSON file.
// If the file does not exist, it creates a new configuration with the empty services list.
func Load(filePath string) (NoPerfection, error) {
	if !strings.HasSuffix(filePath, ".json") {
		return NoPerfection{}, fmt.Errorf("config file %q must end with .json", filePath)
	}

	appConfig := NoPerfection{filePath: filePath}
	myceliumURL := fmt.Sprintf("pkg:json/%s#%s", filepath.Dir(filePath), filepath.Base(filePath))

	data, err := os.ReadFile(filePath)
	if errors.Is(err, fs.ErrNotExist) {
		mycelium, err := json_substrate.Digest(myceliumURL, `{"services":[]}`)
		if err != nil {
			return NoPerfection{}, fmt.Errorf("digest mycelium: %w", err)
		}
		appConfig.mycelium = mycelium
		return appConfig, nil
	}
	if err != nil {
		return NoPerfection{}, fmt.Errorf("os.ReadFile('%s'): %w", filePath, err)
	}

	mycelium, err := json_substrate.Digest(myceliumURL, string(data))
	if err != nil {
		return NoPerfection{}, fmt.Errorf("digest mycelium: %w", err)
	}
	appConfig.mycelium = mycelium

	if err := appConfig.ValidateTopology("pkg:$?*var=services"); err != nil {
		return NoPerfection{}, fmt.Errorf("ValidateTopology: %w", err)
	}

	return appConfig, nil
}

// ValidateTopology validates services, including inline service definitions, and
// checks that every referenced dependency target resolves.
func (a *NoPerfection) ValidateTopology(servicesURL string) error {
	if a == nil {
		return fmt.Errorf("app struct is nil")
	}

	services, err := a.GetServices(servicesURL)
	if err != nil {
		return err
	}

	visiting := make(map[string]bool)
	for i := range services {
		if err := a.validateServiceTopology(&services[i], visiting); err != nil {
			return fmt.Errorf("service %q: %w", services[i].Name, err)
		}
	}

	return a.validatePointedRefs(services)
}

func (a *NoPerfection) validateServiceTopology(service *Service, visiting map[string]bool) error {
	if service == nil {
		return fmt.Errorf("service is nil")
	}
	if service.Name == "" {
		return fmt.Errorf("service name is empty")
	}
	if visiting[service.Name] {
		return fmt.Errorf("cycle detected at service %q", service.Name)
	}
	visiting[service.Name] = true
	defer delete(visiting, service.Name)

	if err := ValidateService(*service); err != nil {
		return err
	}

	for di := range service.HandlerDeps {
		dep := &service.HandlerDeps[di]
		if err := a.validateDepServiceTopology(dep, visiting); err != nil {
			return fmt.Errorf("handler-deps category %q: %w", dep.Name, err)
		}
	}

	for hi := range service.Handlers {
		if service.Handlers[hi] == nil {
			return fmt.Errorf("handler %d is empty", hi)
		}
		handler, ok := service.Handlers[hi].AsIndependentHandler()
		if !ok {
			return fmt.Errorf("handler %d is not an independent handler", hi)
		}

		for di := range handler.CommandDeps {
			dep := &handler.CommandDeps[di]
			if err := a.validateDepServiceTopology(dep, visiting); err != nil {
				return fmt.Errorf("command %q: %w", dep.Name, err)
			}
		}
		if service.Type == ProxyType {
			proxyHandler, ok := service.Handlers[hi].AsProxyHandler()
			if !ok {
				return fmt.Errorf("handler[%d] must be a proxy handler", hi)
			}
			for oi := range proxyHandler.Outbounds {
				target := &proxyHandler.Outbounds[oi]
				if err := ValidateOutboundService(*target); err != nil {
					return fmt.Errorf("handler[%d] outbounds[%d]: %w", hi, oi, err)
				}
			}
		}
		if service.Type == ExtensionType {
			extensionHandler, ok := service.Handlers[hi].AsExtensionHandler()
			if !ok {
				return fmt.Errorf("handler[%d] must be an extension handler", hi)
			}
			for ii := range extensionHandler.Inbounds {
				target := &extensionHandler.Inbounds[ii]
				if err := ValidateInboundService(*target); err != nil {
					return fmt.Errorf("handler[%d] inbounds[%d]: %w", hi, ii, err)
				}
			}
		}
	}
	return nil
}

func (a *NoPerfection) validateDepServiceTopology(dep *DepService, visiting map[string]bool) error {
	if err := ValidateDepService(*dep); err != nil {
		return err
	}

	for i := range dep.Proxies {
		if err := a.validateServicePointerTopology(&dep.Proxies[i], visiting); err != nil {
			return fmt.Errorf("proxies[%d]: %w", i, err)
		}
	}
	for i := range dep.Extensions {
		if err := a.validateServicePointerTopology(&dep.Extensions[i], visiting); err != nil {
			return fmt.Errorf("extensions[%d]: %w", i, err)
		}
	}
	return nil
}

func (a *NoPerfection) validateServicePointerTopology(target *ServicePointer, visiting map[string]bool) error {
	if err := ValidateServicePointer(*target); err != nil {
		return err
	}

	if target.Ref != "" {
		return nil
	}

	service := target.Service
	if err := a.validateServiceTopology(&service, visiting); err != nil {
		return err
	}
	return nil
}

func (a *NoPerfection) validatePointedRefs(services []Service) error {
	for _, service := range services {
		if err := a.validateServiceRefs(service); err != nil {
			return fmt.Errorf("service %q: %w", service.Name, err)
		}
	}
	return nil
}

func (a *NoPerfection) validateServiceRefs(service Service) error {
	for _, dep := range service.HandlerDeps {
		if err := a.validateDepServiceRefs(dep); err != nil {
			return fmt.Errorf("handler-deps category %q: %w", dep.Name, err)
		}
	}
	for hi, handler := range service.Handlers {
		if handler == nil {
			return fmt.Errorf("handler %d is empty", hi)
		}
		baseHandler, ok := handler.AsIndependentHandler()
		if !ok {
			return fmt.Errorf("handler %d is not an independent handler", hi)
		}
		for _, dep := range baseHandler.CommandDeps {
			if err := a.validateDepServiceRefs(dep); err != nil {
				return fmt.Errorf("command %q: %w", dep.Name, err)
			}
		}
		if service.Type == ProxyType {
			proxyHandler, ok := handler.AsProxyHandler()
			if !ok {
				return fmt.Errorf("handler[%d] must be a proxy handler", hi)
			}
			for oi, target := range proxyHandler.Outbounds {
				if err := ValidateOutboundService(target); err != nil {
					return fmt.Errorf("handler[%d] outbounds[%d]: %w", hi, oi, err)
				}
			}
		}
		if service.Type == ExtensionType {
			extensionHandler, ok := handler.AsExtensionHandler()
			if !ok {
				return fmt.Errorf("handler[%d] must be an extension handler", hi)
			}
			for ii, target := range extensionHandler.Inbounds {
				if err := ValidateInboundService(target); err != nil {
					return fmt.Errorf("handler[%d] inbounds[%d]: %w", hi, ii, err)
				}
			}
		}
	}
	return nil
}

func (a *NoPerfection) validateDepServiceRefs(dep DepService) error {
	for _, target := range dep.Proxies {
		if err := a.validateServicePointer(target); err != nil {
			return fmt.Errorf("proxy: %w", err)
		}
	}
	for _, target := range dep.Extensions {
		if err := a.validateServicePointer(target); err != nil {
			return fmt.Errorf("extension: %w", err)
		}
	}
	return nil
}

func (a *NoPerfection) validateServicePointer(target ServicePointer) error {
	if target.Ref != "" {
		serviceName, handlerCategory := target.RefPath()
		if serviceName == "" {
			return fmt.Errorf("dep target service name is empty")
		}
		record, err := a.GetService(serviceName)
		if err != nil {
			return fmt.Errorf("service %q not found: %w", serviceName, err)
		}
		if handlerCategory == "" {
			return nil
		}
		if _, err := record.HandlerByCategory(handlerCategory); err != nil {
			return fmt.Errorf("service %q handler category %q: %w", serviceName, handlerCategory, err)
		}
		return nil
	}

	return a.validateServiceRefs(target.Service)
}

// Save saves the app configuration as JSON into its file path.
func (a NoPerfection) Save() error {
	if len(a.filePath) == 0 {
		return fmt.Errorf("app file path is empty")
	}
	if a.mycelium == nil {
		return fmt.Errorf("topology mycelium not set, call config.Load()")
	}

	raw, err := a.mycelium.Mineralize()
	if err != nil {
		return fmt.Errorf("mycelium.Mineralize: %w", err)
	}

	jsonText, ok := raw.(string)
	if !ok {
		return fmt.Errorf("mycelium.Mineralize returned %T, want string", raw)
	}

	var indented bytes.Buffer
	if err := json.Indent(&indented, []byte(jsonText), "", "  "); err != nil {
		return fmt.Errorf("json.Indent: %w", err)
	}
	indented.WriteByte('\n')

	if err := os.WriteFile(a.filePath, indented.Bytes(), 0600); err != nil {
		return fmt.Errorf("os.WriteFile('%s'): %w", a.filePath, err)
	}

	return nil
}

// GetService resolves a Mushroom URL and returns a single service.
// Plain service names are resolved as pkg:$?*var=services[name:<name>].
func (a *NoPerfection) GetService(mushroomURL string) (Service, error) {
	fruited, err := a.queryMycelium(normalizeServiceURL(mushroomURL))
	if err != nil {
		return Service{}, err
	}

	value, err := unwrapServiceValue(fruited)
	if err != nil {
		return Service{}, fmt.Errorf("GetService(%q): %w", mushroomURL, err)
	}

	service, err := decodeService(value)
	if err != nil {
		return Service{}, fmt.Errorf("GetService(%q): %w", mushroomURL, err)
	}

	return service, nil
}

// GetServices resolves a Mushroom URL and returns the services at that path.
func (a *NoPerfection) GetServices(mushroomURL string) ([]Service, error) {
	fruited, err := a.queryMycelium(mushroomURL)
	if err != nil {
		return nil, err
	}

	services, err := decodeServices(fruited)
	if err != nil {
		return nil, fmt.Errorf("GetServices(%q): %w", mushroomURL, err)
	}

	return services, nil
}

// CountByType returns the number of services resolved by the Mushroom URL.
func (a *NoPerfection) CountByType(mushroomURL string) (int, error) {
	services, err := a.GetServices(mushroomURL)
	if err != nil {
		return 0, err
	}
	return len(services), nil
}

// AddService adds a new service into the services array at parent.
// parent is a dereference Mushroom URL, e.g. pkg:$?*var=services.
func (a *NoPerfection) AddService(record Service, parent string) error {
	if a == nil {
		return fmt.Errorf("app struct is nil")
	}
	if len(record.Name) == 0 {
		return fmt.Errorf("service name is empty")
	}
	if parent == "" {
		return fmt.Errorf("parent URL is empty")
	}

	services, err := a.GetServices(parent)
	if err != nil {
		return fmt.Errorf("GetServices(%q): %w", parent, err)
	}
	if serviceExists(services, record.Name) {
		return fmt.Errorf("service('%s') already exists", record.Name)
	}

	serviceMap, err := encodeServiceMap(record)
	if err != nil {
		return err
	}
	if err := a.mycelium.Graft(parent, serviceMap); err != nil {
		return fmt.Errorf("mycelium.Graft(%q): %w", parent, err)
	}

	return nil
}

// SetService updates an existing service in the services array at parent.
// parent is a dereference Mushroom URL, e.g. pkg:$?*var=services.
func (a *NoPerfection) SetService(record Service, parent string) error {
	if a == nil {
		return fmt.Errorf("app struct is nil")
	}
	if len(record.Name) == 0 {
		return fmt.Errorf("service name is empty")
	}
	if parent == "" {
		return fmt.Errorf("parent URL is empty")
	}

	services, err := a.GetServices(parent)
	if err != nil {
		return fmt.Errorf("GetServices(%q): %w", parent, err)
	}
	if !serviceExists(services, record.Name) {
		return fmt.Errorf("service('%s') not found", record.Name)
	}

	serviceMap, err := encodeServiceMap(record)
	if err != nil {
		return err
	}

	targetURL := fmt.Sprintf("%s[name:%s]", parent, record.Name)
	if err := a.mycelium.Inoculate(targetURL, serviceMap); err != nil {
		return fmt.Errorf("mycelium.Inoculate(%q): %w", targetURL, err)
	}

	return nil
}

// RemoveService removes a service by name from the services array at parent.
// parent is a dereference Mushroom URL, e.g. pkg:$?*var=services.
func (a *NoPerfection) RemoveService(name, parent string) error {
	if a == nil {
		return fmt.Errorf("app struct is nil")
	}
	if len(name) == 0 {
		return fmt.Errorf("service name argument is empty")
	}
	if parent == "" {
		return fmt.Errorf("parent URL is empty")
	}

	services, err := a.GetServices(parent)
	if err != nil {
		return fmt.Errorf("GetServices(%q): %w", parent, err)
	}
	if !serviceExists(services, name) {
		return fmt.Errorf("service('%s') not found", name)
	}

	targetURL := fmt.Sprintf("%s[name:%s]", parent, name)
	if err := a.mycelium.Prune(targetURL); err != nil {
		return fmt.Errorf("mycelium.Prune(%q): %w", targetURL, err)
	}

	return nil
}

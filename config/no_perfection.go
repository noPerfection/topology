// Package config defines the noPerfection application configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// NoPerfection is the configuration of the entire application.
// Consists the supported services.
type NoPerfection struct {
	Services []Service `json:"services"`
	filePath string
}

// Load loads an app configuration from a JSON file.
// If the file does not exist, it creates a new configuration with the empty services list.
func Load(filePath string) (NoPerfection, error) {
	appConfig := NoPerfection{
		Services: make([]Service, 0),
		filePath: filePath,
	}

	data, err := os.ReadFile(filePath)
	if errors.Is(err, fs.ErrNotExist) {
		return appConfig, nil
	}
	if err != nil {
		return NoPerfection{}, fmt.Errorf("os.ReadFile('%s'): %w", filePath, err)
	}

	if err := json.Unmarshal(data, &appConfig); err != nil {
		return NoPerfection{}, fmt.Errorf("json.Unmarshal: %w", err)
	}

	if err := appConfig.ValidateTopology(); err != nil {
		return NoPerfection{}, fmt.Errorf("ValidateTopology: %w", err)
	}

	return appConfig, nil
}

// ValidateTopology validates services, including inline service definitions, and
// checks that every referenced dependency target resolves.
func (a *NoPerfection) ValidateTopology() error {
	if a == nil {
		return fmt.Errorf("app struct is nil")
	}

	visiting := make(map[string]bool)
	for i := range a.Services {
		if err := a.validateServiceTopology(&a.Services[i], visiting); err != nil {
			return fmt.Errorf("service %q: %w", a.Services[i].Name, err)
		}
	}

	return a.validatePointedRefs()
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
		handler := service.Handlers[hi].Handler
		if service.Handlers[hi].ProxyHandler != nil {
			handler = &service.Handlers[hi].ProxyHandler.Handler
		}
		if handler == nil {
			return fmt.Errorf("handler %d is empty", hi)
		}

		for di := range handler.CommandDeps {
			dep := &handler.CommandDeps[di]
			if err := a.validateDepServiceTopology(dep, visiting); err != nil {
				return fmt.Errorf("command %q: %w", dep.Name, err)
			}
		}
		if service.Type == ProxyType && service.Handlers[hi].ProxyHandler != nil {
			proxyHandler := service.Handlers[hi].ProxyHandler
			for oi := range proxyHandler.Outbounds {
				target := &proxyHandler.Outbounds[oi]
				if err := a.validateServicePointerTopology(target, visiting); err != nil {
					return fmt.Errorf("outbound %q: %w", handler.Category, err)
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

func (a *NoPerfection) validatePointedRefs() error {
	for _, service := range a.Services {
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
	for _, handler := range service.Handlers {
		baseHandler := handler.AsHandler()
		for _, dep := range baseHandler.CommandDeps {
			if err := a.validateDepServiceRefs(dep); err != nil {
				return fmt.Errorf("command %q: %w", dep.Name, err)
			}
		}
		if service.Type == ProxyType && handler.ProxyHandler != nil {
			for _, target := range handler.ProxyHandler.Outbounds {
				if err := a.validateServicePointer(target); err != nil {
					return fmt.Errorf("outbound %q: %w", baseHandler.Category, err)
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

	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("json.MarshalIndent: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(a.filePath, data, 0600); err != nil {
		return fmt.Errorf("os.WriteFile('%s'): %w", a.filePath, err)
	}

	return nil
}

// GetService returns a service by name from the app configuration.
// If not found, return an error.
func (a *NoPerfection) GetService(name string) (Service, error) {
	for i := range a.Services {
		if a.Services[i].Name == name {
			return a.Services[i], nil
		}
	}

	return Service{}, fmt.Errorf("service('%s') not found", name)
}

// GetByType returns the first service of the given type from the app configuration.
// If the service type is invalid or no service is found, return an error.
func (a *NoPerfection) GetByType(serviceType Type) (*Service, error) {
	if err := ValidateServiceType(serviceType); err != nil {
		return nil, fmt.Errorf("ValidateServiceType: %w", err)
	}

	for i := range a.Services {
		if a.Services[i].Type == serviceType {
			return &a.Services[i], nil
		}
	}

	return nil, fmt.Errorf("service of '%s' type not found", serviceType)
}

// FilterByType returns all services of the given type from the app configuration.
// If the service type is invalid or no services are found, return an error.
func (a *NoPerfection) FilterByType(serviceType Type) ([]*Service, error) {
	if err := ValidateServiceType(serviceType); err != nil {
		return nil, fmt.Errorf("ValidateServiceType: %w", err)
	}

	services := make([]*Service, 0)
	for i := range a.Services {
		if a.Services[i].Type == serviceType {
			services = append(services, &a.Services[i])
		}
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("no services of '%s' type found", serviceType)
	}
	return services, nil
}

// CountByType returns the amount of services of the given type.
func (a *NoPerfection) CountByType(serviceType Type) int {
	count := 0
	for i := range a.Services {
		if a.Services[i].Type == serviceType {
			count++
		}
	}

	return count
}

// SetService sets a new service into the configuration.
func (a *NoPerfection) SetService(record Service) error {
	if a == nil {
		return fmt.Errorf("app struct is nil")
	}

	found := false
	for i, old := range a.Services {
		if old.Name == record.Name {
			found = true
			a.Services[i] = record
			break
		}
	}
	if !found {
		a.Services = append(a.Services, record)
	}

	return nil
}

// RemoveService removes a service by name from the app configuration.
func (a *NoPerfection) RemoveService(name string) error {
	if a == nil {
		return fmt.Errorf("app struct is nil")
	}
	if len(name) == 0 {
		return fmt.Errorf("service name argument is empty")
	}

	for i := range a.Services {
		if a.Services[i].Name == name {
			a.Services = append(a.Services[:i], a.Services[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("service('%s') not found", name)
}

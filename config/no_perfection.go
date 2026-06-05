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

	if err := appConfig.Normalize(); err != nil {
		return NoPerfection{}, fmt.Errorf("Normalize: %w", err)
	}

	return appConfig, nil
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

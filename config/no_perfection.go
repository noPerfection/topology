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
}

// normalizeServiceURL maps a service lookup argument to a dereference Mushroom URL.
// Mushroom URLs are returned unchanged; a plain symbol is expanded to a root name filter.
//
//	symbol:      "auth_proxy"  →  "*pkg:$?var=services[name:auth_proxy]"
//	mushroomURL: "*pkg:$?var=services[name:auth_proxy]"  →  unchanged
func normalizeServiceURL(mushroomURL string) string {
	if strings.HasPrefix(mushroomURL, "pkg:") || strings.HasPrefix(mushroomURL, "*pkg:") {
		return mushroomURL
	}
	return fmt.Sprintf("*pkg:$?var=services[name:%s]", mushroomURL)
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
// If the file does not exist, it seeds an empty services list and loads via Root.
func Load(filePath string) (NoPerfection, error) {
	if !strings.HasSuffix(filePath, ".json") {
		return NoPerfection{}, fmt.Errorf("config file %q must end with .json", filePath)
	}

	myceliumURL := fmt.Sprintf("pkg:json/%s#%s", filepath.Dir(filePath), filepath.Base(filePath))

	loaded := true
	if _, err := os.Stat(filePath); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
			return NoPerfection{}, fmt.Errorf("os.MkdirAll(%q): %w", filepath.Dir(filePath), err)
		}
		if err := os.WriteFile(filePath, []byte("{\n  \"services\": []\n}\n"), 0600); err != nil {
			return NoPerfection{}, fmt.Errorf("os.WriteFile('%s'): %w", filePath, err)
		}
		loaded = false
	} else if err != nil {
		return NoPerfection{}, fmt.Errorf("os.Stat('%s'): %w", filePath, err)
	}

	mycelium, err := json_substrate.Root(myceliumURL)
	if err != nil {
		return NoPerfection{}, fmt.Errorf("json_substrate.Root(%q): %w", myceliumURL, err)
	}

	appConfig := NoPerfection{mycelium: mycelium}

	if loaded {
		if err := appConfig.validateTopology("*pkg:$?var=services"); err != nil {
			return NoPerfection{}, fmt.Errorf("validateTopology: %w", err)
		}
	}

	return appConfig, nil
}

func (a *NoPerfection) validateTopology(servicesURL string) error {
	if a == nil {
		return fmt.Errorf("app struct is nil")
	}

	services, err := a.GetServices(servicesURL)
	if err != nil {
		return err
	}

	for i := range services {
		if err := ValidateService(services[i]); err != nil {
			return fmt.Errorf("service %q: %w", services[i].Name, err)
		}
	}

	return a.validatePointedRefs(services)
}

func (a *NoPerfection) validatePointedRefs(services []Service) error {
	visiting := make(map[string]bool)
	for _, service := range services {
		if err := a.validateServiceRefs(service, visiting); err != nil {
			return fmt.Errorf("service %q: %w", service.Name, err)
		}
	}
	return nil
}

func (a *NoPerfection) validateServiceRefs(service Service, visiting map[string]bool) error {
	if service.Name != "" {
		if visiting[service.Name] {
			return fmt.Errorf("cycle detected at service %q", service.Name)
		}
		visiting[service.Name] = true
		defer delete(visiting, service.Name)
	}

	for _, dep := range service.HandlerDeps {
		if err := a.validateDepServiceRefs(dep, visiting); err != nil {
			return fmt.Errorf("handler-deps category %q: %w", dep.Name, err)
		}
	}
	for _, handler := range service.Handlers {
		baseHandler, ok := handler.AsIndependentHandler()
		if !ok {
			continue
		}
		for _, dep := range baseHandler.CommandDeps {
			if err := a.validateDepServiceRefs(dep, visiting); err != nil {
				return fmt.Errorf("command %q: %w", dep.Name, err)
			}
		}
	}
	return nil
}

func (a *NoPerfection) validateDepServiceRefs(dep DepService, visiting map[string]bool) error {
	for _, target := range dep.Proxies {
		if err := a.validateServicePointer(target, visiting); err != nil {
			return fmt.Errorf("proxy: %w", err)
		}
	}
	for _, target := range dep.Extensions {
		if err := a.validateServicePointer(target, visiting); err != nil {
			return fmt.Errorf("extension: %w", err)
		}
	}
	return nil
}

func (a *NoPerfection) validateServicePointer(target ServicePointer, visiting map[string]bool) error {
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

	return a.validateServiceRefs(target.Service, visiting)
}

// Save saves the app configuration as JSON into its file path.
func (a NoPerfection) Save() error {
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

	substrate, ok := (*a.mycelium.Substrate()).(*json_substrate.Substrate)
	if !ok {
		return fmt.Errorf("mycelium substrate is %T, want *json_substrate.Substrate", *a.mycelium.Substrate())
	}

	hypha, err := a.mycelium.Soil().Hypha(a.mycelium.MushroomURL())
	if err != nil {
		return fmt.Errorf("soil.Hypha(%q): %w", a.mycelium.MushroomURL(), err)
	}

	var indented bytes.Buffer
	if err := json.Indent(&indented, []byte(jsonText), "", "  "); err != nil {
		return fmt.Errorf("json.Indent: %w", err)
	}
	indented.WriteByte('\n')

	if err := substrate.Sow(hypha, indented.String()); err != nil {
		return fmt.Errorf("substrate.Sow: %w", err)
	}

	filePath := filepath.Join(hypha.PackageID, hypha.ModuleID)
	if err := os.Chmod(filePath, 0600); err != nil {
		return fmt.Errorf("os.Chmod('%s'): %w", filePath, err)
	}

	return nil
}

// GetService resolves a Mushroom URL and returns a single service.
// Plain service names are resolved as *pkg:$?var=services[name:<name>].
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
// parent is a dereference Mushroom URL, e.g. *pkg:$?var=services.
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
// parent is a dereference Mushroom URL, e.g. *pkg:$?var=services.
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
// parent is a dereference Mushroom URL, e.g. *pkg:$?var=services.
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

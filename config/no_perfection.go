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

	"github.com/ahmetson/mushroom"
	"github.com/ahmetson/mushroom/substrates/json_substrate"
)

// NoPerfection is the configuration of the entire application.
// Consists the supported services.
type NoPerfection struct {
	mycelium *json_substrate.Mycelium
}

// toHypha converts mushroomURL (link or dereference) into a full github.com/ahmetson/mushroom.Hypha
// wildcards are filled against the root mycelium.
// Plain symbols expand to *pkg:$?var=services[name:<symbol>].
func (a *NoPerfection) toHypha(mushroomURL string) (mushroom.Hypha, error) {
	if a == nil {
		return mushroom.Hypha{}, fmt.Errorf("app struct is nil")
	}
	if a.mycelium == nil {
		return mushroom.Hypha{}, fmt.Errorf("topology mycelium not set, call config.Load()")
	}
	if mushroomURL == "" {
		return mushroom.Hypha{}, fmt.Errorf("mushroom url is empty")
	}

	hypha, err := a.mycelium.Soil().Hypha(mushroomURL, a.mycelium.MyceliumURL())
	if err != nil {
		return mushroom.Hypha{}, fmt.Errorf("soil.Hypha(%q): %w", mushroomURL, err)
	}
	if !hypha.URL {
		hypha, err = a.mycelium.Soil().Hypha(
			fmt.Sprintf("*pkg:$?var=services[name:%s]", mushroomURL),
			a.mycelium.MyceliumURL(),
		)
		if err != nil {
			return mushroom.Hypha{}, fmt.Errorf("soil.Hypha(%q): %w", mushroomURL, err)
		}
	}

	return hypha, nil
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

func (a *NoPerfection) decodeService(value any) (Service, error) {
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

	if a != nil {
		service.noPerf = a
		service.mycelium = &a.mycelium
	}

	return service, nil
}

func (a *NoPerfection) decodeServices(value any) ([]Service, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("mushroom url is fetching wrong data: expected array, got %T", value)
	}

	services := make([]Service, 0, len(items))
	for _, item := range items {
		service, err := a.decodeService(item)
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

// unwrapServiceValue normalizes a single mycelium result for decodeService.
// Filtered queries return an array; a direct path returns one object.
//
//	*pkg:$?var=services[name:auth_proxy]  →  [{...service...}]
//	unwrapServiceValue(...)                 →  {...service...}
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

// Load loads an app configuration from a symbolic file path or a Mushroom link URL.
//
// Symbolic paths are plain filesystem paths ending in .json (e.g. "noPerfection.json",
// "/etc/app/noPerfection.json"). They are turned into a json mycelium link:
//
//	noPerfection.json  →  pkg:json/.#noPerfection.json
//
// Mushroom URLs must be links (not dereferences), use substrate type json, refer to a
// module (no ?var= resource path), and end with a .json module id
// (e.g. pkg:json/tmp#app.json).
//
// If the backing file does not exist, Load seeds an empty services list. When the file
// already exists, Load validates the topology graph.
func Load(mushroomURL string) (NoPerfection, error) {
	linkURL, filePath, err := resolveLoadMyceliumURL(mushroomURL)
	if err != nil {
		return NoPerfection{}, err
	}

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

	mycelium, err := json_substrate.Root(linkURL)
	if err != nil {
		return NoPerfection{}, fmt.Errorf("json_substrate.Root(%q): %w", linkURL, err)
	}

	appConfig := NoPerfection{
		mycelium: mycelium,
	}

	if loaded {
		if err := appConfig.validateTopology("*pkg:$?var=services"); err != nil {
			return NoPerfection{}, fmt.Errorf("validateTopology: %w", err)
		}
	}

	return appConfig, nil
}

// resolveLoadMyceliumURL maps a Load argument to a json mycelium link and filesystem path.
func resolveLoadMyceliumURL(arg string) (linkURL string, filePath string, err error) {
	if arg == "" {
		return "", "", fmt.Errorf("mushroom url is empty")
	}

	if strings.HasPrefix(arg, "*pkg:") {
		return "", "", fmt.Errorf("Load mushroom URL %q must be a link, not a dereference", arg)
	}

	if strings.HasPrefix(arg, "pkg:") {
		soil := &mushroom.Soil{}
		hypha, parseErr := soil.Hypha(arg)
		if parseErr != nil {
			return "", "", fmt.Errorf("soil.Hypha(%q): %w", arg, parseErr)
		}
		if hypha.Dereference {
			return "", "", fmt.Errorf("Load mushroom URL %q must be a link, not a dereference", arg)
		}
		if hypha.Type != "json" {
			return "", "", fmt.Errorf("Load mushroom URL %q type must be json, got %q", arg, hypha.Type)
		}
		if hypha.ModuleID == "" {
			return "", "", fmt.Errorf("Load mushroom URL %q must include a module", arg)
		}
		if !strings.HasSuffix(hypha.ModuleID, ".json") {
			return "", "", fmt.Errorf("Load mushroom URL %q module must end with .json", arg)
		}
		if hypha.PackageID == "" {
			return "", "", fmt.Errorf("Load mushroom URL %q must include a package", arg)
		}
		if hypha.ResourceKind != "" {
			return "", "", fmt.Errorf("Load mushroom URL %q must refer to a module, not a resource path", arg)
		}
		return hypha.String(), filepath.Join(hypha.PackageID, hypha.ModuleID), nil
	}

	if !strings.HasSuffix(arg, ".json") {
		return "", "", fmt.Errorf("config file %q must end with .json", arg)
	}

	link := fmt.Sprintf("pkg:json/%s#%s", filepath.Dir(arg), filepath.Base(arg))
	return link, arg, nil
}

func (a *NoPerfection) validateTopology(servicesURL string) error {
	if a == nil {
		return fmt.Errorf("app struct is nil")
	}

	services, err := a.GetServices(servicesURL)
	if err != nil {
		return err
	}

	visiting := make(map[string]bool)
	for i := range services {
		if err := ValidateService(services[i]); err != nil {
			return fmt.Errorf("service %q: %w", services[i].Name, err)
		}
		if err := a.validateServiceTopology(services[i], visiting); err != nil {
			return fmt.Errorf("service %q: %w", services[i].Name, err)
		}
	}
	return nil
}

func (a *NoPerfection) validateServiceTopology(service Service, visiting map[string]bool) error {
	if service.Name != "" {
		if visiting[service.Name] {
			return fmt.Errorf("cycle detected at service %q", service.Name)
		}
		visiting[service.Name] = true
		defer delete(visiting, service.Name)
	}

	for _, dep := range service.HandlerDeps {
		for _, link := range dep.Proxies {
			if err := a.validateDepLink(link); err != nil {
				return fmt.Errorf("handler-deps category %q: proxy: %w", dep.Name, err)
			}
		}
		for _, link := range dep.Extensions {
			if err := a.validateDepLink(link); err != nil {
				return fmt.Errorf("handler-deps category %q: extension: %w", dep.Name, err)
			}
		}
	}
	for _, handler := range service.Handlers {
		baseHandler, ok := handler.AsIndependentHandler()
		if !ok {
			continue
		}
		for _, dep := range baseHandler.CommandDeps {
			for _, link := range dep.Proxies {
				if err := a.validateDepLink(link); err != nil {
					return fmt.Errorf("command %q: proxy: %w", dep.Name, err)
				}
			}
			for _, link := range dep.Extensions {
				if err := a.validateDepLink(link); err != nil {
					return fmt.Errorf("command %q: extension: %w", dep.Name, err)
				}
			}
		}
		if service.Type == ProxyType {
			proxyHandler, ok := handler.AsProxyHandler()
			if !ok {
				continue
			}
			if err := validateProxyForwardOutbounds(proxyHandler); err != nil {
				return fmt.Errorf("handler %q: %w", baseHandler.Category, err)
			}
		}
	}
	return nil
}

func validateProxyForwardOutbounds(proxyHandler ProxyHandler) error {
	for route, outboundRef := range proxyHandler.Forward {
		if !proxyHandlerHasOutboundRef(proxyHandler, outboundRef) {
			return fmt.Errorf("forward route %q: outbound %q is not listed in outbounds", route, outboundRef)
		}
	}
	return nil
}

func parseForwardRef(ref string) (service string, handlerCategory string, err error) {
	if ref == "" {
		return "", "", fmt.Errorf("forward outbound ref is empty")
	}
	if strings.HasSuffix(ref, "/") {
		return "", "", fmt.Errorf("forward outbound ref %q has empty handler category", ref)
	}
	if strings.Contains(ref, "//") {
		return "", "", fmt.Errorf("forward outbound ref %q has empty path segment", ref)
	}

	service, handlerCategory, ok := strings.Cut(ref, "/")
	if !ok {
		return ref, "", nil
	}
	if service == "" {
		return "", "", fmt.Errorf("forward outbound service name is empty")
	}
	if handlerCategory == "" {
		return "", "", fmt.Errorf("forward outbound handler category is empty")
	}
	return service, handlerCategory, nil
}

func proxyHandlerHasOutboundRef(proxyHandler ProxyHandler, ref string) bool {
	serviceName, handlerCategory, err := parseForwardRef(ref)
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

func (a *NoPerfection) validateDepLink(link string) error {
	if _, _, err := a.ResolveDep(link); err != nil {
		return fmt.Errorf("dep link %q: %w", link, err)
	}
	return nil
}

// NoHandlerError reports that a resolved service does not contain the requested handler category.
type NoHandlerError struct {
	Service  string
	Category string
}

func (e *NoHandlerError) Error() string {
	return fmt.Sprintf("handler of %q category not found on service %q", e.Category, e.Service)
}

// ResolveDep resolves a dependency mushroom URL to a service and handler category.
// The URL may be a link or dereference pointing at a root service.
// When the URL carries an additional "category" property, that handler category
// is used; otherwise DefaultCategory is used.
func (a *NoPerfection) ResolveDep(mushroomURL string) (Service, string, error) {
	if a == nil {
		return Service{}, "", fmt.Errorf("app struct is nil")
	}
	if a.mycelium == nil {
		return Service{}, "", fmt.Errorf("topology mycelium not set, call config.Load()")
	}
	if mushroomURL == "" {
		return Service{}, "", fmt.Errorf("mushroom url is empty")
	}

	hypha, err := a.toHypha(mushroomURL)
	if err != nil {
		return Service{}, "", err
	}

	category := depCategory(hypha)
	hypha.AdditionalProps = nil

	service, err := a.GetService(hypha.AsDereference().String())
	if err != nil {
		return Service{}, "", err
	}

	// Make sure the category exists too.
	if _, err := service.HandlerByCategory(category); err != nil {
		return Service{}, "", &NoHandlerError{Service: service.Name, Category: category}
	}

	return service, category, nil
}

func depCategory(hypha mushroom.Hypha) string {
	if category, ok := hypha.AdditionalProps["category"]; ok && category != "" {
		return category
	}
	return DefaultCategory
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

	hypha := a.mycelium.MyceliumURL()

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
	hypha, err := a.toHypha(mushroomURL)
	if err != nil {
		return Service{}, err
	}

	fruited, err := a.queryMycelium(hypha.String())
	if err != nil {
		return Service{}, err
	}

	value, err := unwrapServiceValue(fruited)
	if err != nil {
		return Service{}, fmt.Errorf("GetService(%q): %w", mushroomURL, err)
	}

	service, err := a.decodeService(value)
	if err != nil {
		return Service{}, fmt.Errorf("GetService(%q): %w", mushroomURL, err)
	}

	hypha.AdditionalProps = nil
	service.mushroomURL = hypha.AsLink()

	return service, nil
}

// GetHandler resolves a Mushroom URL and returns a single handler.
// When the URL resolves to a service, the handler with DefaultCategory is returned.
func (a *NoPerfection) GetHandler(mushroomURL string) (Handler, error) {
	fruited, err := a.queryMycelium(mushroomURL)
	if err != nil {
		return nil, err
	}

	handler, err := decodeHandler(fruited)
	if err == nil {
		return handler, nil
	}

	// URL resolved to a service (often an array from a name filter); unwrap before decode.
	value, err := unwrapServiceValue(fruited)
	if err != nil {
		return nil, fmt.Errorf("GetHandler(%q): handler not found", mushroomURL)
	}

	service, err := a.decodeService(value)
	if err != nil {
		return nil, fmt.Errorf("GetHandler(%q): handler not found", mushroomURL)
	}

	handler, err = service.HandlerByCategory(DefaultCategory)
	if err != nil {
		return nil, fmt.Errorf("GetHandler(%q): %w", mushroomURL, err)
	}

	return handler, nil
}

// GetServices resolves a Mushroom URL and returns the services at that path.
func (a *NoPerfection) GetServices(mushroomURL string) ([]Service, error) {
	hypha, err := a.toHypha(mushroomURL)
	if err != nil {
		return nil, err
	}

	fruited, err := a.queryMycelium(hypha.String())
	if err != nil {
		return nil, err
	}

	services, err := a.decodeServices(fruited)
	if err != nil {
		return nil, fmt.Errorf("GetServices(%q): %w", mushroomURL, err)
	}

	listLink := hypha.AsLink()
	listLink.AdditionalProps = nil
	parent, _ := listLink.ParentResource()

	for i := range services {
		child, err := parent.ChildResource("[name:" + services[i].Name + "]")
		if err != nil {
			return nil, fmt.Errorf("GetServices(%q): service %q mushroom url: %w", mushroomURL, services[i].Name, err)
		}
		services[i].mushroomURL = child
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

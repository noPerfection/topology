package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noPerfection/protocol/message"
)

const testServicesParent = "*pkg:$?var=services"

func addTestService(t *testing.T, a *NoPerfection, record Service) {
	t.Helper()
	if err := a.AddService(record, testServicesParent); err != nil {
		t.Fatalf("AddService: %v", err)
	}
}

func listTestServices(t *testing.T, a *NoPerfection) []Service {
	t.Helper()
	services, err := a.GetServices(testServicesParent)
	if err != nil {
		t.Fatalf("GetServices: %v", err)
	}
	return services
}

func loadServices(t *testing.T, services []Service) (NoPerfection, error) {
	t.Helper()

	filePath := filepath.Join(t.TempDir(), "test.json")
	payload, err := json.Marshal(map[string][]Service{"services": services})
	if err != nil {
		return NoPerfection{}, err
	}
	if err := os.WriteFile(filePath, payload, 0600); err != nil {
		return NoPerfection{}, err
	}

	return Load(filePath)
}

func mustLoadServices(t *testing.T, services []Service) NoPerfection {
	t.Helper()

	app, err := loadServices(t, services)
	if err != nil {
		t.Fatalf("loadServices: %v", err)
	}
	return app
}

func mustLoadEmpty(t *testing.T) NoPerfection {
	t.Helper()

	app, err := Load(filepath.Join(t.TempDir(), "test.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return app
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()

	a, err := Load(filepath.Join(dir, "missing.json"))
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	services := listTestServices(t, &a)
	if len(services) != 0 {
		t.Fatalf("len(Services) = %d, want 0", len(services))
	}
}

func TestGetService(t *testing.T) {
	a := mustLoadEmpty(t)
	sample := Service{Name: "api", Type: IndependentType}
	addTestService(t, &a, sample)

	found, err := a.GetService("api")
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if found.Name != "api" {
		t.Fatalf("Name = %q, want api", found.Name)
	}

	if _, err := a.GetService("missing"); err == nil {
		t.Fatal("GetService missing service returned nil error")
	}
}

func TestGetServiceByMushroomURL(t *testing.T) {
	a := mustLoadEmpty(t)
	sample := Service{Name: "api", Type: IndependentType}
	addTestService(t, &a, sample)

	found, err := a.GetService("*pkg:$?var=services[name:api]")
	if err != nil {
		t.Fatalf("GetService by mushroom url: %v", err)
	}
	if found.Name != "api" {
		t.Fatalf("Name = %q, want api", found.Name)
	}
}

func TestGetHandler(t *testing.T) {
	service := Service{
		Name:         "auth_proxy",
		Type:         ProxyType,
		StartCommand: "true",
		Handlers: []Handler{
			ProxyHandler{
				IndependentHandler: IndependentHandler{
					Type:     ReplierType,
					Category: ServiceManagerCategory,
					Endpoint: message.NewEndpoint("tmp/auth_manager", 0),
				},
			},
			ProxyHandler{
				IndependentHandler: IndependentHandler{
					Type:     ReplierType,
					Category: DefaultCategory,
					Endpoint: message.NewEndpoint("auth_main", 4301),
				},
			},
		},
	}
	app := mustLoadServices(t, []Service{service})

	handler, err := app.GetHandler("*pkg:$?var=services[name:auth_proxy]")
	if err != nil {
		t.Fatalf("GetHandler by service url: %v", err)
	}
	ind, ok := handler.AsIndependentHandler()
	if !ok || ind.Category != DefaultCategory || ind.Endpoint.Port != 4301 {
		t.Fatalf("handler = %#v, want %q on port 4301", handler, DefaultCategory)
	}

	handler, err = app.GetHandler("*pkg:$?var=services[name:auth_proxy].handlers[category:main]")
	if err != nil {
		t.Fatalf("GetHandler by handler url: %v", err)
	}
	ind, ok = handler.AsIndependentHandler()
	if !ok || ind.Category != DefaultCategory {
		t.Fatalf("handler = %#v, want %q", handler, DefaultCategory)
	}

	if _, err := app.GetHandler("*pkg:$?var=services[name:missing]"); err == nil {
		t.Fatal("GetHandler missing service returned nil error")
	}
}

func TestGetServices(t *testing.T) {
	a := mustLoadEmpty(t)
	services := []Service{
		{Name: "api", Type: IndependentType},
		{Name: "worker", Type: IndependentType},
		{Name: "proxy", Type: ProxyType},
	}
	for _, s := range services {
		if err := a.AddService(s, testServicesParent); err != nil {
			t.Fatalf("AddService: %v", err)
		}
	}

	if _, err := a.GetServices("*pkg:$?var=services[type:Extension]"); err == nil {
		t.Fatal("GetServices with missing type returned nil error")
	}

	found, err := a.GetServices("*pkg:$?var=services[type:Independent]")
	if err != nil {
		t.Fatalf("GetServices independent: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("len(found) = %d, want 2", len(found))
	}
	if found[0].Name != "api" {
		t.Fatalf("first service = %q, want api", found[0].Name)
	}
	if found[1].Name != "worker" {
		t.Fatalf("second service = %q, want worker", found[1].Name)
	}
}

func TestCountByType(t *testing.T) {
	a := mustLoadEmpty(t)
	services := []Service{
		{Name: "api", Type: IndependentType},
		{Name: "worker", Type: IndependentType},
		{Name: "proxy", Type: ProxyType},
	}
	for _, s := range services {
		if err := a.AddService(s, testServicesParent); err != nil {
			t.Fatalf("AddService: %v", err)
		}
	}

	count, err := a.CountByType("*pkg:$?var=services[type:Independent]")
	if err != nil {
		t.Fatalf("CountByType independent: %v", err)
	}
	if count != 2 {
		t.Fatalf("CountByType independent = %d, want 2", count)
	}

	count, err = a.CountByType("*pkg:$?var=services[type:Extension]")
	if err == nil {
		t.Fatalf("CountByType extension = %d, want error", count)
	}

	count, err = a.CountByType("*pkg:$?var=services[type:invalid]")
	if err == nil {
		t.Fatalf("CountByType invalid = %d, want error", count)
	}
}

func TestSetService(t *testing.T) {
	a := mustLoadEmpty(t)
	first := Service{Name: "api", Type: IndependentType}
	second := Service{Name: "proxy", Type: ProxyType}

	if err := a.SetService(first, testServicesParent); err == nil {
		t.Fatal("SetService missing service returned nil error")
	}
	addTestService(t, &a, first)
	addTestService(t, &a, second)
	services := listTestServices(t, &a)
	if len(services) != 2 {
		t.Fatalf("len(Services) = %d, want 2", len(services))
	}

	updated := first
	updated.StartCommand = "go run ./cmd/api"
	if err := a.SetService(updated, testServicesParent); err != nil {
		t.Fatalf("SetService update: %v", err)
	}
	services = listTestServices(t, &a)
	if len(services) != 2 {
		t.Fatalf("len(Services) after update = %d, want 2", len(services))
	}

	found, err := a.GetService("api")
	if err != nil {
		t.Fatalf("GetService updated: %v", err)
	}
	if found.StartCommand != "go run ./cmd/api" {
		t.Fatalf("StartCommand = %q, want go run ./cmd/api", found.StartCommand)
	}
}

func TestRemoveService(t *testing.T) {
	a := mustLoadEmpty(t)
	first := Service{Name: "api", Type: IndependentType}
	second := Service{Name: "proxy", Type: ProxyType}
	addTestService(t, &a, first)
	addTestService(t, &a, second)

	if err := a.RemoveService("", testServicesParent); err == nil {
		t.Fatal("RemoveService with empty name returned nil error")
	}
	if err := a.RemoveService("missing", testServicesParent); err == nil {
		t.Fatal("RemoveService with missing service returned nil error")
	}

	if err := a.RemoveService("api", testServicesParent); err != nil {
		t.Fatalf("RemoveService: %v", err)
	}
	services := listTestServices(t, &a)
	if len(services) != 1 {
		t.Fatalf("len(Services) = %d, want 1", len(services))
	}
	if services[0].Name != "proxy" {
		t.Fatalf("remaining service = %q, want proxy", services[0].Name)
	}
}

func TestLoadSave(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "app.json")
	original, err := Load(filePath)
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	sample := Service{
		Name: "api",
		Type: IndependentType,
		Handlers: []Handler{
			IndependentHandler{
				Type:     ReplierType,
				Category: "api",
				Endpoint: message.NewEndpoint("api_1", 4101),
			},
		},
	}
	addTestService(t, &original, sample)

	if err := original.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if !jsonLooksIndented(data) {
		t.Fatalf("written JSON is not indented: %s", string(data))
	}

	loaded, err := Load(filePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	services := listTestServices(t, &loaded)
	if len(services) != 1 {
		t.Fatalf("len(Services) = %d, want 1", len(services))
	}
	if services[0].Name != "api" {
		t.Fatalf("Name = %q, want api", services[0].Name)
	}
	handler, ok := services[0].Handlers[0].AsIndependentHandler()
	if !ok || handler.Endpoint.Port != 4101 {
		t.Fatalf("Port = %d, want 4101", handler.Endpoint.Port)
	}
}

func TestSaveWithoutFilePath(t *testing.T) {
	if err := (NoPerfection{}).Save(); err == nil {
		t.Fatal("Save without file path returned nil error")
	}
}

func TestOperationsRequireLoad(t *testing.T) {
	a := NoPerfection{}

	if _, err := a.GetService("api"); err == nil {
		t.Fatal("GetService without Load returned nil error")
	}
	if err := a.AddService(Service{Name: "api", Type: IndependentType}, testServicesParent); err == nil {
		t.Fatal("AddService without Load returned nil error")
	}
}

func TestValidateTopologyMissingRef(t *testing.T) {
	_, err := loadServices(t, []Service{
		{
			Type: IndependentType,
			Name: "api",
			Handlers: []Handler{
				IndependentHandler{
					Type:     ReplierType,
					Category: "api",
					Endpoint: message.NewEndpoint("api_1", 4101),
					CommandDeps: []DepService{
						{
							Name:    "route",
							Proxies: []string{"pkg:$?var=services[name:missing_proxy]"},
						},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("Load with missing ref returned nil error")
	}
}

func TestValidateTopologyMissingHandlerDepRef(t *testing.T) {
	_, err := loadServices(t, []Service{
		{
			Type: IndependentType,
			Name: "api",
			Handlers: []Handler{
				IndependentHandler{
					Type:     ReplierType,
					Category: "api",
					Endpoint: message.NewEndpoint("api_1", 4101),
				},
			},
			HandlerDeps: []DepService{
				{
					Name:    "api",
					Proxies: []string{"pkg:$?var=services[name:missing_proxy]"},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("Load with missing handler-deps ref returned nil error")
	}
}

func TestValidateTopologyRejectsHandlerDepLink(t *testing.T) {
	_, err := loadServices(t, []Service{
		{
			Type: IndependentType,
			Name: "api",
			Handlers: []Handler{
				IndependentHandler{
					Type:     ReplierType,
					Category: "api",
					Endpoint: message.NewEndpoint("api_1", 4101),
					CommandDeps: []DepService{
						{
							Name:    "route",
							Proxies: []string{"pkg:$?var=services[name:auth_proxy].handlers[category:main]"},
						},
					},
				},
			},
		},
		{
			Type: ProxyType,
			Name: "auth_proxy",
			Handlers: []Handler{
				ProxyHandler{
					IndependentHandler: IndependentHandler{
						Type:     ReplierType,
						Category: "main",
						Endpoint: message.NewEndpoint("auth_1", 4301),
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("Load with handler dep link returned nil error")
	}
	if !strings.Contains(err.Error(), "value is not a service") {
		t.Fatalf("error = %q, want service decode failure", err.Error())
	}
}

func TestResolveDepServiceLink(t *testing.T) {
	app := mustLoadServices(t, []Service{
		{
			Type: ProxyType,
			Name: "auth_proxy",
			Handlers: []Handler{
				ProxyHandler{
					IndependentHandler: IndependentHandler{
						Type:     ReplierType,
						Category: "main",
						Endpoint: message.NewEndpoint("auth_1", 4301),
					},
				},
			},
		},
	})

	service, category, err := app.ResolveDep("pkg:$?var=services[name:auth_proxy]")
	if err != nil {
		t.Fatalf("ResolveDep: %v", err)
	}
	if service.Name != "auth_proxy" {
		t.Fatalf("service = %q, want auth_proxy", service.Name)
	}
	if category != DefaultCategory {
		t.Fatalf("category = %q, want %q", category, DefaultCategory)
	}
}

func TestResolveDepWithCategoryProperty(t *testing.T) {
	app := mustLoadServices(t, []Service{
		{
			Type: ProxyType,
			Name: "auth_proxy",
			Handlers: []Handler{
				ProxyHandler{
					IndependentHandler: IndependentHandler{
						Type:     ReplierType,
						Category: "auth-proxy",
						Endpoint: message.NewEndpoint("auth_1", 4301),
					},
				},
			},
		},
	})

	service, category, err := app.ResolveDep("pkg:$?var=services[name:auth_proxy]&category=auth-proxy")
	if err != nil {
		t.Fatalf("ResolveDep: %v", err)
	}
	if service.Name != "auth_proxy" {
		t.Fatalf("service = %q, want auth_proxy", service.Name)
	}
	if category != "auth-proxy" {
		t.Fatalf("category = %q, want auth-proxy", category)
	}
}

func TestResolveDepNoHandler(t *testing.T) {
	app := mustLoadServices(t, []Service{
		{
			Type: ProxyType,
			Name: "auth_proxy",
			Handlers: []Handler{
				ProxyHandler{
					IndependentHandler: IndependentHandler{
						Type:     ReplierType,
						Category: "main",
						Endpoint: message.NewEndpoint("auth_1", 4301),
					},
				},
			},
		},
	})

	_, _, err := app.ResolveDep("pkg:$?var=services[name:auth_proxy]&category=missing")
	if err == nil {
		t.Fatal("ResolveDep missing handler returned nil error")
	}
	var noHandler *NoHandlerError
	if !errors.As(err, &noHandler) {
		t.Fatalf("error = %T, want *NoHandlerError", err)
	}
	if noHandler.Service != "auth_proxy" || noHandler.Category != "missing" {
		t.Fatalf("NoHandlerError = %#v, want auth_proxy/missing", noHandler)
	}
}

func TestResolveDepRejectsHandlerURL(t *testing.T) {
	app := mustLoadServices(t, []Service{
		{
			Type: ProxyType,
			Name: "auth_proxy",
			Handlers: []Handler{
				ProxyHandler{
					IndependentHandler: IndependentHandler{
						Type:     ReplierType,
						Category: "main",
						Endpoint: message.NewEndpoint("auth_1", 4301),
					},
				},
			},
		},
	})

	_, _, err := app.ResolveDep("pkg:$?var=services[name:auth_proxy].handlers[category:main]")
	if err == nil {
		t.Fatal("ResolveDep handler url returned nil error")
	}
	if !strings.Contains(err.Error(), "value is not a service") {
		t.Fatalf("error = %q, want service decode failure", err.Error())
	}
}

func TestValidateTopologyServiceDepLink(t *testing.T) {
	app := mustLoadServices(t, []Service{
		{
			Type: IndependentType,
			Name: "api",
			Handlers: []Handler{
				IndependentHandler{
					Type:     ReplierType,
					Category: "api",
					Endpoint: message.NewEndpoint("api_1", 4101),
					CommandDeps: []DepService{
						{
							Name:    "route",
							Proxies: []string{"pkg:$?var=services[name:auth_proxy]"},
						},
					},
				},
			},
		},
		{
			Type: ProxyType,
			Name: "auth_proxy",
			Handlers: []Handler{
				ProxyHandler{
					IndependentHandler: IndependentHandler{
						Type:     ReplierType,
						Category: "main",
						Endpoint: message.NewEndpoint("auth_1", 4301),
					},
				},
			},
		},
	})

	if err := app.validateTopology(testServicesParent); err != nil {
		t.Fatalf("ValidateTopology with service dep link: %v", err)
	}

	services := listTestServices(t, &app)
	handler, ok := services[0].Handlers[0].AsIndependentHandler()
	if !ok {
		t.Fatal("handler is not an independent handler")
	}
	target := handler.CommandDeps[0].Proxies[0]
	if target != "pkg:$?var=services[name:auth_proxy]" {
		t.Fatalf("link = %q, want auth_proxy service link", target)
	}
}

func TestLoadWithDepLinks(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.json")
	jsonData := []byte(`{
  "services": [
    {
      "type": "Independent",
      "name": "api",
      "handler-deps": [
        {
          "name": "api",
          "proxies": ["pkg:$?var=services[name:auth_proxy]&category=auth"]
        }
      ],
      "handlers": [
        {
          "type": "Replier",
          "category": "api",
          "endpoint": {"id": "api_1", "port": 4101},
          "command-deps": [
            {
              "name": "route",
              "proxies": ["pkg:$?var=services[name:auth_proxy]&category=auth"]
            }
          ]
        }
      ]
    },
    {
      "type": "Proxy",
      "name": "auth_proxy",
      "handlers": [
        {
          "type": "Replier",
          "category": "auth",
          "endpoint": {"id": "auth_1", "port": 4301},
          "outbounds": []
        }
      ]
    }
  ]
}`)

	if err := os.WriteFile(filePath, jsonData, 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	app, err := Load(filePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := app.GetService("auth_proxy"); err != nil {
		t.Fatalf("GetService auth_proxy: %v", err)
	}

	services := listTestServices(t, &app)
	handler, ok := services[0].Handlers[0].AsIndependentHandler()
	if !ok {
		t.Fatal("handler is not an independent handler")
	}
	dep := handler.CommandDeps[0]
	if len(dep.Proxies) != 1 {
		t.Fatalf("len(Proxies) = %d, want 1", len(dep.Proxies))
	}
	if dep.Proxies[0] != "pkg:$?var=services[name:auth_proxy]&category=auth" {
		t.Fatalf("proxy link = %q, want auth_proxy link with category", dep.Proxies[0])
	}
	handlerDep := services[0].HandlerDeps[0]
	if handlerDep.Proxies[0] != "pkg:$?var=services[name:auth_proxy]&category=auth" {
		t.Fatalf("handler-deps proxy link = %q, want auth_proxy link with category", handlerDep.Proxies[0])
	}
}

func TestLoadProxyChainExample(t *testing.T) {
	app, err := Load(filepath.Join("examples", "app-proxy-chain.json"))
	if err != nil {
		t.Fatalf("Load app-proxy-chain example: %v", err)
	}

	for _, name := range []string{
		"auth_proxy",
		"audit_proxy",
		"user_service",
	} {
		if _, err := app.GetService(name); err != nil {
			t.Fatalf("GetService(%q): %v", name, err)
		}
	}
}

func TestFacadeProxyChain(t *testing.T) {
	app, err := Load(filepath.Join("examples", "app-proxy-chain.json"))
	if err != nil {
		t.Fatalf("Load app-proxy-chain example: %v", err)
	}

	main, err := app.GetService("main")
	if err != nil {
		t.Fatalf("GetService(main): %v", err)
	}

	endpoint, err := main.Facade("main", "authorize")
	if err != nil {
		t.Fatalf("Facade(main, authorize): %v", err)
	}
	if endpoint.Port != 4301 {
		t.Fatalf("Facade(main, authorize) port = %d, want 4301", endpoint.Port)
	}

	endpoint, err = main.Facade("public-api", "authorize")
	if err != nil {
		t.Fatalf("Facade(public-api, authorize): %v", err)
	}
	if endpoint.Port != 4301 {
		t.Fatalf("Facade(public-api, authorize) port = %d, want 4301", endpoint.Port)
	}

	userService, err := app.GetService("user_service")
	if err != nil {
		t.Fatalf("GetService(user_service): %v", err)
	}
	endpoint, err = userService.Facade("user-service", "")
	if err != nil {
		t.Fatalf("Facade(user-service): %v", err)
	}
	if endpoint.Port != 4401 {
		t.Fatalf("Facade(user-service) port = %d, want 4401", endpoint.Port)
	}
}

func TestFacadeRequiresTopology(t *testing.T) {
	service := Service{
		Name: "orphan",
		Type: IndependentType,
		Handlers: []Handler{
			IndependentHandler{
				Type:     ReplierType,
				Category: DefaultCategory,
				Endpoint: message.NewEndpoint("orphan_1", 4101),
			},
		},
	}

	_, err := service.Facade("main", "authorize")
	if err == nil {
		t.Fatal("Facade without topology returned nil error")
	}
	if !strings.Contains(err.Error(), "without topology, can't get its facade") {
		t.Fatalf("error = %q, want topology error", err.Error())
	}
}

func TestLoadMushroomURL(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.json")
	if err := os.WriteFile(filePath, []byte("{\n  \"services\": []\n}\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	app, err := Load(fmt.Sprintf("pkg:json/%s#app.json", dir))
	if err != nil {
		t.Fatalf("Load mushroom URL: %v", err)
	}
	if _, err := app.GetServices(testServicesParent); err != nil {
		t.Fatalf("GetServices: %v", err)
	}
}

func TestLoadRejectsDereferenceMushroomURL(t *testing.T) {
	_, err := Load("*pkg:json/tmp#app.json")
	if err == nil {
		t.Fatal("Load dereference URL: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "link") {
		t.Fatalf("Load dereference URL error = %q, want link mention", err.Error())
	}
}

func TestLoadRejectsNonJSONMushroomType(t *testing.T) {
	_, err := Load("pkg:yaml/tmp#app.json")
	if err == nil {
		t.Fatal("Load non-json type: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "type must be json") {
		t.Fatalf("Load non-json type error = %q", err.Error())
	}
}

func TestLoadRejectsMushroomURLWithoutJSONModule(t *testing.T) {
	_, err := Load("pkg:json/tmp#app.yaml")
	if err == nil {
		t.Fatal("Load non-json module: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "module must end with .json") {
		t.Fatalf("Load non-json module error = %q", err.Error())
	}
}

func TestLoadRejectsMushroomURLWithResourcePath(t *testing.T) {
	_, err := Load("pkg:json/tmp#app.json?var=services")
	if err == nil {
		t.Fatal("Load resource path: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "module, not a resource path") {
		t.Fatalf("Load resource path error = %q", err.Error())
	}
}

func jsonLooksIndented(data []byte) bool {
	for _, b := range data {
		if b == '\n' {
			return true
		}
	}
	return false
}

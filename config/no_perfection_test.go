package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/noPerfection/protocol/message"
)

const testServicesParent = "pkg:$?*var=services"

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

	found, err := a.GetService("pkg:$?*var=services[name:api]")
	if err != nil {
		t.Fatalf("GetService by mushroom url: %v", err)
	}
	if found.Name != "api" {
		t.Fatalf("Name = %q, want api", found.Name)
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

	if _, err := a.GetServices("pkg:$?*var=services[type:Extension]"); err == nil {
		t.Fatal("GetServices with missing type returned nil error")
	}

	found, err := a.GetServices("pkg:$?*var=services[type:Independent]")
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

	count, err := a.CountByType("pkg:$?*var=services[type:Independent]")
	if err != nil {
		t.Fatalf("CountByType independent: %v", err)
	}
	if count != 2 {
		t.Fatalf("CountByType independent = %d, want 2", count)
	}

	count, err = a.CountByType("pkg:$?*var=services[type:Extension]")
	if err == nil {
		t.Fatalf("CountByType extension = %d, want error", count)
	}

	count, err = a.CountByType("pkg:$?*var=services[type:invalid]")
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

func TestValidateTopologyInlineService(t *testing.T) {
	app := mustLoadServices(t, []Service{
		{
			Type: IndependentType,
			Name: "public_api",
			Handlers: []Handler{
				IndependentHandler{
					Type:     ReplierType,
					Category: "public-api",
					Endpoint: message.NewEndpoint("public_1", 4101),
					CommandDeps: []DepService{
						{
							Name: "call-user-api",
							Proxies: []ServicePointer{
								ServiceTarget(Service{Name: "nested_proxy", Type: ProxyType}),
							},
						},
					},
				},
			},
		},
	})
	publicAPI, err := app.GetService("public_api")
	if err != nil {
		t.Fatalf("GetService public_api: %v", err)
	}
	handler, ok := publicAPI.Handlers[0].AsIndependentHandler()
	if !ok {
		t.Fatal("handler is not an independent handler")
	}
	handler.CommandDeps[0].Proxies[0].Service.Handlers = []Handler{
		ProxyHandler{
			IndependentHandler: IndependentHandler{
				Type:     ReplierType,
				Category: "nested",
				Endpoint: message.NewEndpoint("nested_1", 4201),
			},
		},
	}
	publicAPI.Handlers[0] = handler
	if err := app.SetService(publicAPI, testServicesParent); err != nil {
		t.Fatalf("SetService public_api: %v", err)
	}

	if err := app.ValidateTopology(testServicesParent); err != nil {
		t.Fatalf("ValidateTopology: %v", err)
	}

	if _, err := app.GetService("nested_proxy"); err == nil {
		t.Fatal("GetService nested_proxy returned nil error for inline service")
	}
	if _, err := app.GetService("public_api"); err != nil {
		t.Fatalf("GetService public_api: %v", err)
	}
}

func TestValidateTopologyServiceHandlerDeps(t *testing.T) {
	app := mustLoadServices(t, []Service{
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
					Name: "api",
					Proxies: []ServicePointer{
						ServiceTarget(Service{
							Type: ProxyType,
							Name: "inline_proxy",
							Handlers: []Handler{
								ProxyHandler{
									IndependentHandler: IndependentHandler{
										Type:     ReplierType,
										Category: "inline",
										Endpoint: message.NewEndpoint("inline_1", 4201),
									},
								},
							},
						}),
					},
				},
			},
		},
	})

	if err := app.ValidateTopology(testServicesParent); err != nil {
		t.Fatalf("ValidateTopology: %v", err)
	}
	if _, err := app.GetService("inline_proxy"); err == nil {
		t.Fatal("GetService inline_proxy returned nil error for inline service")
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
							Proxies: []ServicePointer{RefTarget("missing_proxy")},
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
					Proxies: []ServicePointer{RefTarget("missing_proxy")},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("Load with missing handler-deps ref returned nil error")
	}
}

func TestValidateTopologyRefPathWithHandlerCategory(t *testing.T) {
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
							Proxies: []ServicePointer{RefTarget("auth_proxy", "main")},
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

	if err := app.ValidateTopology(testServicesParent); err != nil {
		t.Fatalf("ValidateTopology with ref path: %v", err)
	}

	services := listTestServices(t, &app)
	handler, ok := services[0].Handlers[0].AsIndependentHandler()
	if !ok {
		t.Fatal("handler is not an independent handler")
	}
	target := handler.CommandDeps[0].Proxies[0]
	if target.Ref != "auth_proxy/main" {
		t.Fatalf("ref path = %q, want auth_proxy/main", target.Ref)
	}
	serviceName, category := target.RefPath()
	if serviceName != "auth_proxy" || category != "main" {
		t.Fatalf("Ref() = (%q, %q), want (auth_proxy, main)", serviceName, category)
	}
}

func TestValidateTopologyRefPathMissingHandlerCategory(t *testing.T) {
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
							Proxies: []ServicePointer{RefTarget("auth_proxy", "missing")},
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
		t.Fatal("Load with missing ref path handler category returned nil error")
	}
}

func TestLoadWithMixedDepTargets(t *testing.T) {
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
          "proxies": ["auth_proxy"]
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
              "proxies": [
                "auth_proxy",
                {
                  "type": "Proxy",
                  "name": "inline_proxy",
                  "handlers": [
                    {
                      "type": "Replier",
                      "category": "inline",
                      "endpoint": {"id": "inline_1", "port": 4201},
                      "outbounds": []
                    }
                  ]
                }
              ]
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
	if _, err := app.GetService("inline_proxy"); err == nil {
		t.Fatal("GetService inline_proxy returned nil error for inline service")
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
	if len(dep.Proxies) != 2 {
		t.Fatalf("len(Proxies) = %d, want 2", len(dep.Proxies))
	}
	if dep.Proxies[0].Ref != "auth_proxy" {
		t.Fatalf("first proxy path = %q, want auth_proxy", dep.Proxies[0].Ref)
	}
	if dep.Proxies[1].Service.Name != "inline_proxy" {
		t.Fatalf("second proxy inline = %#v", dep.Proxies[1].Service)
	}
	handlerDep := services[0].HandlerDeps[0]
	if handlerDep.Proxies[0].Ref != "auth_proxy" {
		t.Fatalf("handler-deps first proxy path = %q, want auth_proxy", handlerDep.Proxies[0].Ref)
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
	for _, name := range []string{
		"inline_service_target",
		"inline_proxy_target",
		"inline_extension_target",
		"inline_extension_proxy_target",
	} {
		if _, err := app.GetService(name); err == nil {
			t.Fatalf("GetService(%q) returned nil error for inline service", name)
		}
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

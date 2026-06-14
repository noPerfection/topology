package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/noPerfection/protocol/message"
)

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()

	a, err := Load(filepath.Join(dir, "missing.json"))
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	if a.Services == nil {
		t.Fatal("Services is nil")
	}
	if len(a.Services) != 0 {
		t.Fatalf("len(Services) = %d, want 0", len(a.Services))
	}
}

func TestGetService(t *testing.T) {
	a := NoPerfection{}
	sample := Service{Name: "api", Type: IndependentType}
	if err := a.AddService(sample); err != nil {
		t.Fatalf("AddService: %v", err)
	}

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

func TestGetByType(t *testing.T) {
	a := NoPerfection{}
	services := []Service{
		{Name: "api", Type: IndependentType},
		{Name: "worker", Type: IndependentType},
		{Name: "proxy", Type: ProxyType},
	}
	for _, s := range services {
		if err := a.AddService(s); err != nil {
			t.Fatalf("AddService: %v", err)
		}
	}

	if _, err := a.GetByType(Type("invalid")); err == nil {
		t.Fatal("GetByType with invalid type returned nil error")
	}
	if _, err := a.GetByType(ExtensionType); err == nil {
		t.Fatal("GetByType with missing type returned nil error")
	}

	found, err := a.GetByType(IndependentType)
	if err != nil {
		t.Fatalf("GetByType independent: %v", err)
	}
	if found.Name != "api" {
		t.Fatalf("Name = %q, want api", found.Name)
	}
}

func TestFilterByType(t *testing.T) {
	a := NoPerfection{}
	services := []Service{
		{Name: "api", Type: IndependentType},
		{Name: "worker", Type: IndependentType},
		{Name: "proxy", Type: ProxyType},
	}
	for _, s := range services {
		if err := a.AddService(s); err != nil {
			t.Fatalf("AddService: %v", err)
		}
	}

	if _, err := a.FilterByType(Type("invalid")); err == nil {
		t.Fatal("FilterByType with invalid type returned nil error")
	}
	if _, err := a.FilterByType(ExtensionType); err == nil {
		t.Fatal("FilterByType with missing type returned nil error")
	}

	found, err := a.FilterByType(IndependentType)
	if err != nil {
		t.Fatalf("FilterByType independent: %v", err)
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
	a := NoPerfection{}
	services := []Service{
		{Name: "api", Type: IndependentType},
		{Name: "worker", Type: IndependentType},
		{Name: "proxy", Type: ProxyType},
	}
	for _, s := range services {
		if err := a.AddService(s); err != nil {
			t.Fatalf("AddService: %v", err)
		}
	}

	if count := a.CountByType(IndependentType); count != 2 {
		t.Fatalf("CountByType independent = %d, want 2", count)
	}
	if count := a.CountByType(ExtensionType); count != 0 {
		t.Fatalf("CountByType extension = %d, want 0", count)
	}
	if count := a.CountByType(Type("invalid")); count != 0 {
		t.Fatalf("CountByType invalid = %d, want 0", count)
	}
}

func TestSetService(t *testing.T) {
	a := NoPerfection{}
	first := Service{Name: "api", Type: IndependentType}
	second := Service{Name: "proxy", Type: ProxyType}

	if err := a.SetService(first); err == nil {
		t.Fatal("SetService missing service returned nil error")
	}
	if err := a.AddService(first); err != nil {
		t.Fatalf("AddService first: %v", err)
	}
	if err := a.AddService(second); err != nil {
		t.Fatalf("AddService second: %v", err)
	}
	if len(a.Services) != 2 {
		t.Fatalf("len(Services) = %d, want 2", len(a.Services))
	}

	updated := first
	updated.StartCommand = "go run ./cmd/api"
	if err := a.SetService(updated); err != nil {
		t.Fatalf("SetService update: %v", err)
	}
	if len(a.Services) != 2 {
		t.Fatalf("len(Services) after update = %d, want 2", len(a.Services))
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
	a := NoPerfection{}
	first := Service{Name: "api", Type: IndependentType}
	second := Service{Name: "proxy", Type: ProxyType}
	if err := a.AddService(first); err != nil {
		t.Fatalf("AddService first: %v", err)
	}
	if err := a.AddService(second); err != nil {
		t.Fatalf("AddService second: %v", err)
	}

	if err := a.RemoveService(""); err == nil {
		t.Fatal("RemoveService with empty name returned nil error")
	}
	if err := a.RemoveService("missing"); err == nil {
		t.Fatal("RemoveService with missing service returned nil error")
	}

	if err := a.RemoveService("api"); err != nil {
		t.Fatalf("RemoveService: %v", err)
	}
	if len(a.Services) != 1 {
		t.Fatalf("len(Services) = %d, want 1", len(a.Services))
	}
	if a.Services[0].Name != "proxy" {
		t.Fatalf("remaining service = %q, want proxy", a.Services[0].Name)
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
	if err := original.AddService(sample); err != nil {
		t.Fatalf("AddService: %v", err)
	}

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
	if len(loaded.Services) != 1 {
		t.Fatalf("len(Services) = %d, want 1", len(loaded.Services))
	}
	if loaded.Services[0].Name != "api" {
		t.Fatalf("Name = %q, want api", loaded.Services[0].Name)
	}
	handler, ok := loaded.Services[0].Handlers[0].AsIndependentHandler()
	if !ok || handler.Endpoint.Port != 4101 {
		t.Fatalf("Port = %d, want 4101", handler.Endpoint.Port)
	}
}

func TestSaveWithoutFilePath(t *testing.T) {
	if err := (NoPerfection{}).Save(); err == nil {
		t.Fatal("Save without file path returned nil error")
	}
}

func TestValidateTopologyInlineService(t *testing.T) {
	app := NoPerfection{
		Services: []Service{
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
		},
	}
	handler, ok := app.Services[0].Handlers[0].AsIndependentHandler()
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
	app.Services[0].Handlers[0] = handler

	if err := app.ValidateTopology(); err != nil {
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
	app := NoPerfection{
		Services: []Service{
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
		},
	}

	if err := app.ValidateTopology(); err != nil {
		t.Fatalf("ValidateTopology: %v", err)
	}
	if _, err := app.GetService("inline_proxy"); err == nil {
		t.Fatal("GetService inline_proxy returned nil error for inline service")
	}
}

func TestValidateTopologyMissingRef(t *testing.T) {
	app := NoPerfection{
		Services: []Service{
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
		},
	}

	if err := app.ValidateTopology(); err == nil {
		t.Fatal("ValidateTopology with missing ref returned nil error")
	}
}

func TestValidateTopologyMissingHandlerDepRef(t *testing.T) {
	app := NoPerfection{
		Services: []Service{
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
		},
	}

	if err := app.ValidateTopology(); err == nil {
		t.Fatal("ValidateTopology with missing handler-deps ref returned nil error")
	}
}

func TestValidateTopologyRefPathWithHandlerCategory(t *testing.T) {
	app := NoPerfection{
		Services: []Service{
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
		},
	}

	if err := app.ValidateTopology(); err != nil {
		t.Fatalf("ValidateTopology with ref path: %v", err)
	}

	handler, ok := app.Services[0].Handlers[0].AsIndependentHandler()
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
	app := NoPerfection{
		Services: []Service{
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
		},
	}

	if err := app.ValidateTopology(); err == nil {
		t.Fatal("ValidateTopology with missing ref path handler category returned nil error")
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

	handler, ok := app.Services[0].Handlers[0].AsIndependentHandler()
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
	handlerDep := app.Services[0].HandlerDeps[0]
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

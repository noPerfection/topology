package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/noPerfection/protocol/message"
)

func sr(service Service) ServiceRecord {
	return NewServiceRecord(service)
}

func TestNormalizeInlineService(t *testing.T) {
	app := NoPerfection{
		Services: []ServiceRecord{
			sr(Service{
				Type: IndependentType,
				Name: "public_api",
				Handlers: []Handler{
					{
						Type:     ReplierType,
						Category: "public-api",
						Endpoint: message.NewEndpoint("public_1", 4101),
						CommandDeps: []DepService{
							{
								Name: "call-user-api",
								Proxies: []DepTarget{
									ServiceTarget(*New("nested_proxy", ProxyType)),
								},
							},
						},
					},
				},
			}),
		},
	}
	app.Services[0].Handlers[0].CommandDeps[0].Proxies[0].ServiceRecord.Handlers = []Handler{
		{
			Type:     ReplierType,
			Category: "nested",
			Endpoint: message.NewEndpoint("nested_1", 4201),
		},
	}

	if err := app.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	if _, err := app.GetService("nested_proxy"); err != nil {
		t.Fatalf("GetService nested_proxy: %v", err)
	}
	if _, err := app.GetService("public_api"); err != nil {
		t.Fatalf("GetService public_api: %v", err)
	}
}

func TestNormalizeServiceHandlerDeps(t *testing.T) {
	app := NoPerfection{
		Services: []ServiceRecord{
			sr(Service{
				Type: IndependentType,
				Name: "api",
				Handlers: []Handler{
					{
						Type:     ReplierType,
						Category: "api",
						Endpoint: message.NewEndpoint("api_1", 4101),
					},
				},
				HandlerDeps: []DepService{
					{
						Name: "api",
						Proxies: []DepTarget{
							ServiceTarget(Service{
								Type: ProxyType,
								Name: "inline_proxy",
								Handlers: []Handler{
									{
										Type:     ReplierType,
										Category: "inline",
										Endpoint: message.NewEndpoint("inline_1", 4201),
									},
								},
							}),
						},
					},
				},
			}),
		},
	}

	if err := app.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if _, err := app.GetService("inline_proxy"); err != nil {
		t.Fatalf("GetService inline_proxy: %v", err)
	}
}

func TestNormalizeMissingRef(t *testing.T) {
	app := NoPerfection{
		Services: []ServiceRecord{
			sr(Service{
				Type: IndependentType,
				Name: "api",
				Handlers: []Handler{
					{
						Type:     ReplierType,
						Category: "api",
						Endpoint: message.NewEndpoint("api_1", 4101),
						CommandDeps: []DepService{
							{
								Name:    "route",
								Proxies: []DepTarget{RefTarget("missing_proxy")},
							},
						},
					},
				},
			}),
		},
	}

	if err := app.Normalize(); err == nil {
		t.Fatal("Normalize with missing ref returned nil error")
	}
}

func TestNormalizeMissingHandlerDepRef(t *testing.T) {
	app := NoPerfection{
		Services: []ServiceRecord{
			sr(Service{
				Type: IndependentType,
				Name: "api",
				Handlers: []Handler{
					{
						Type:     ReplierType,
						Category: "api",
						Endpoint: message.NewEndpoint("api_1", 4101),
					},
				},
				HandlerDeps: []DepService{
					{
						Name:    "api",
						Proxies: []DepTarget{RefTarget("missing_proxy")},
					},
				},
			}),
		},
	}

	if err := app.Normalize(); err == nil {
		t.Fatal("Normalize with missing handler-deps ref returned nil error")
	}
}

func TestNormalizeRefPathWithHandlerCategory(t *testing.T) {
	app := NoPerfection{
		Services: []ServiceRecord{
			sr(Service{
				Type: IndependentType,
				Name: "api",
				Handlers: []Handler{
					{
						Type:     ReplierType,
						Category: "api",
						Endpoint: message.NewEndpoint("api_1", 4101),
						CommandDeps: []DepService{
							{
								Name:    "route",
								Proxies: []DepTarget{RefTarget("auth_proxy", "main")},
							},
						},
					},
				},
			}),
			sr(Service{
				Type: ProxyType,
				Name: "auth_proxy",
				Handlers: []Handler{
					{
						Type:     ReplierType,
						Category: "main",
						Endpoint: message.NewEndpoint("auth_1", 4301),
					},
				},
			}),
		},
	}

	if err := app.Normalize(); err != nil {
		t.Fatalf("Normalize with ref path: %v", err)
	}

	target := app.Services[0].Handlers[0].CommandDeps[0].Proxies[0]
	if target.Ref != "auth_proxy/main" {
		t.Fatalf("ref path = %q, want auth_proxy/main", target.Ref)
	}
	serviceName, category := target.RefPath()
	if serviceName != "auth_proxy" || category != "main" {
		t.Fatalf("Ref() = (%q, %q), want (auth_proxy, main)", serviceName, category)
	}
}

func TestNormalizeRefPathMissingHandlerCategory(t *testing.T) {
	app := NoPerfection{
		Services: []ServiceRecord{
			sr(Service{
				Type: IndependentType,
				Name: "api",
				Handlers: []Handler{
					{
						Type:     ReplierType,
						Category: "api",
						Endpoint: message.NewEndpoint("api_1", 4101),
						CommandDeps: []DepService{
							{
								Name:    "route",
								Proxies: []DepTarget{RefTarget("auth_proxy", "missing")},
							},
						},
					},
				},
			}),
			sr(Service{
				Type: ProxyType,
				Name: "auth_proxy",
				Handlers: []Handler{
					{
						Type:     ReplierType,
						Category: "main",
						Endpoint: message.NewEndpoint("auth_1", 4301),
					},
				},
			}),
		},
	}

	if err := app.Normalize(); err == nil {
		t.Fatal("Normalize with missing ref path handler category returned nil error")
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
                      "endpoint": {"id": "inline_1", "port": 4201}
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
          "endpoint": {"id": "auth_1", "port": 4301}
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
	if _, err := app.GetService("inline_proxy"); err != nil {
		t.Fatalf("GetService inline_proxy: %v", err)
	}
	if _, err := app.GetService("auth_proxy"); err != nil {
		t.Fatalf("GetService auth_proxy: %v", err)
	}

	dep := app.Services[0].Handlers[0].CommandDeps[0]
	if len(dep.Proxies) != 2 {
		t.Fatalf("len(Proxies) = %d, want 2", len(dep.Proxies))
	}
	if dep.Proxies[0].Ref != "auth_proxy" {
		t.Fatalf("first proxy path = %q, want auth_proxy", dep.Proxies[0].Ref)
	}
	if dep.Proxies[1].Proxy == nil || dep.Proxies[1].Proxy.Name != "inline_proxy" {
		t.Fatalf("second proxy inline = %#v", dep.Proxies[1].Proxy)
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
		"inline_service_target",
		"inline_proxy_target",
		"inline_extension_target",
		"inline_extension_proxy_target",
		"user_service",
	} {
		if _, err := app.GetService(name); err != nil {
			t.Fatalf("GetService(%q): %v", name, err)
		}
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeInlineService(t *testing.T) {
	app := SdsService{
		Services: []Service{
			{
				Type: IndependentType,
				Name: "public_api",
				Handlers: []Handler{
					{
						Type:     ReplierType,
						Category: "public-api",
						Socket:   Socket{Id: "public_1", Port: 4101},
						CommandDeps: []CommandDep{
							{
								Command: "call-user-api",
								Proxies: []DepTarget{
									InlineTarget(*New("nested_proxy", ProxyType)),
								},
							},
						},
					},
				},
			},
		},
	}
	app.Services[0].Handlers[0].CommandDeps[0].Proxies[0].Inline.Handlers = []Handler{
		{
			Type:     ReplierType,
			Category: "nested",
			Socket:   Socket{Id: "nested_1", Port: 4201},
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

func TestNormalizeMissingRef(t *testing.T) {
	app := SdsService{
		Services: []Service{
			{
				Type: IndependentType,
				Name: "api",
				Handlers: []Handler{
					{
						Type:     ReplierType,
						Category: "api",
						Socket:   Socket{Id: "api_1", Port: 4101},
						CommandDeps: []CommandDep{
							{
								Command: "route",
								Proxies: []DepTarget{RefTarget("missing_proxy")},
							},
						},
					},
				},
			},
		},
	}

	if err := app.Normalize(); err == nil {
		t.Fatal("Normalize with missing ref returned nil error")
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
      "handlers": [
        {
          "type": "Replier",
          "category": "api",
          "socket": {"id": "api_1", "port": 4101},
          "command-deps": [
            {
              "command": "route",
              "proxies": [
                "auth_proxy",
                {
                  "type": "Proxy",
                  "name": "inline_proxy",
                  "handlers": [
                    {
                      "type": "Replier",
                      "category": "inline",
                      "socket": {"id": "inline_1", "port": 4201}
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
          "socket": {"id": "auth_1", "port": 4301}
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
		t.Fatalf("first proxy ref = %q, want auth_proxy", dep.Proxies[0].Ref)
	}
	if dep.Proxies[1].Inline == nil || dep.Proxies[1].Inline.Name != "inline_proxy" {
		t.Fatalf("second proxy inline = %#v", dep.Proxies[1].Inline)
	}
}

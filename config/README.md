# noPerfection Configuration

This module provides Go types and JSON helpers for an application configuration made of services.

It is a static data library only:

- `config` defines service metadata (`Service`, `Handler`, `Endpoint`, `DepService`, `DepTarget`)
- `config` defines the top-level `NoPerfection` struct (`services: [...]`), `Load`, and `NoPerfection.Save`

There is no topology config server, engine, or client API in this module.

## App structure

```json
{
  "services": [
    {
      "type": "Independent",
      "name": "public_api",
      "handler-deps": [
        {
          "name": "public-api",
          "proxies": ["auth_proxy", "audit_proxy"]
        }
      ],
      "handlers": [
        {
          "type": "Replier",
          "category": "public-api",
          "endpoint": {
            "id": "public_1",
            "port": 4101
          }
        }
      ]
    }
  ]
}
```

See [examples/app-proxy-chain.json](examples/app-proxy-chain.json) for a full proxy-chain example.

## Usage

```go
import (
    "github.com/noPerfection/protocol/message"
    config "github.com/noPerfection/topology/config"
)

a, err := config.Load("app.json")
if err != nil {
    panic(err)
}

record, err := a.GetService("public_api")
if err != nil {
    panic(err)
}

updated := record
updated.Handlers = append(updated.Handlers, config.Handler{
    Type:     config.ReplierType,
    Category: "public-api",
    Endpoint: message.NewEndpoint("public_2", 4102),
})
if err := a.SetService(updated); err != nil {
    panic(err)
}

if err := a.Save(); err != nil {
    panic(err)
}
```

## Service Types

Use `config.New(name, serviceType)` to create a service skeleton, then fill handlers and command dependency metadata.

Supported service types:

- `Independent`
- `Proxy`
- `Extension`

Supported handler types:

- `SyncReplier`
- `Replier`
- `Publisher`
- `Pair`

Each handler must define a `category`, which consumers can use to group and classify handlers.

`port` is optional on an endpoint; omitted means `0`. Services only need bootstrap metadata when a zero-port endpoint requires it:

- `port: 0` with an id that does not start with `tmp/` is treated as inproc and requires `module-url`
- `port: 0` with an id that starts with `tmp` is treated as an IPC endpoint and requires `start-command`
- non-zero ports do not require either `module-url` or `start-command`

Each `handler-deps` or `command-deps` entry must have a `name` and at least one routing target: `proxies` and/or `extensions`. In `handler-deps`, `name` is the handler category. In `command-deps`, `name` is the command name.

Each entry in `proxies` or `extensions` is a `DepTarget`. A target is exactly one of:

- a **ref** string (`DepTarget.Ref`)
- a **service record** object (`DepTarget.ServiceRecord`) containing either a service or proxy

`config.Load` calls `Normalize()` to register inline targets and verify references. JSON stays compact: each target is one value, not an object with separate `ref` / `service` / `proxy` keys.

### DepTarget `Ref`

`Ref` is the stored reference path when the dependency points at an existing service in `services`. In JSON it is a single string. In Go it is `DepTarget.Ref`.

Format:

- `service_name` — depend on the service (any handler on that service)
- `service_name/handler_category` — depend on one specific handler category on that service

Rules:

| Ref | Valid | Notes |
|-----|-------|-------|
| `"auth_proxy"` | yes | service name only |
| `"auth_proxy/main"` | yes | service + handler category |
| `""` | no | empty ref |
| `"auth_proxy/"` | no | trailing slash, empty handler category |
| `"/main"` | no | empty service name |
| `"auth_proxy//main"` | no | empty path segment |

Only the first `/` splits service and handler category. There is no deeper path syntax.

After `Load` / `Normalize()`:

- the service name must exist in `services`
- if a handler category is present, that service must define a handler with that `category`

Go helpers:

```go
// Build a ref target in code.
config.RefTarget("auth_proxy")              // Ref: "auth_proxy"
config.RefTarget("auth_proxy", "main")      // Ref: "auth_proxy/main"

target := config.RefTarget("auth_proxy", "main")
serviceName, handlerCategory := target.RefPath()
// serviceName == "auth_proxy", handlerCategory == "main"

target.Name() // always the service name: "auth_proxy"
```

`RefPath()` parses `DepTarget.Ref` into `(serviceName, handlerCategory)`. When the ref has no handler segment, `handlerCategory` is `""`. `Name()` returns the service name for ref and service-record targets.

Example JSON:

```json
"proxies": [
  "auth_proxy",
  "auth_proxy/auth-proxy",
  {
    "type": "Extension",
    "name": "inline_service_target",
    "handlers": [ ... ]
  },
  {
    "type": "Proxy",
    "name": "inline_proxy_target",
    "handlers": [ ... ]
  }
],
"extensions": [
  "user_service",
  "user_service/user-service",
  {
    "type": "Extension",
    "name": "inline_extension_target",
    "handlers": [ ... ]
  },
  {
    "type": "Proxy",
    "name": "inline_extension_proxy_target",
    "handlers": [ ... ]
  }
]
```

See [examples/app-proxy-chain.json](examples/app-proxy-chain.json): the `dep_target_catalog` service lists all four target forms in both `proxies` and `extensions`. The rest of the file shows a smaller realistic chain.

Proxy chains can be declared in service-level `handler-deps` metadata or per-handler `command-deps` metadata. Terminal services that only receive routed traffic do not need either section.

## Packages removed from this module

Previous versions included a dev topology layer (`engine`, `handler`, `client`, `watch`) for serving config over noPerfection sockets. That topology API has been removed. Consumers should load JSON with `config.Load` and save it with `NoPerfection.Save`.

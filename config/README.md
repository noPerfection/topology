# noPerfection Configuration

This module provides Go types and JSON helpers for an application configuration made of services.

It is a static data library only:

- `config` defines service metadata (`Service`, `Handler`, `Endpoint`, `CommandDep`, `DepTarget`)
- `config` defines the top-level `NoPerfection` struct (`services: [...]`), `Load`, and `NoPerfection.Save`

There is no runtime config server, engine, or client API in this module.

## App structure

```json
{
  "services": [
    {
      "type": "Independent",
      "name": "public_api",
      "handlers": [
        {
          "type": "Replier",
          "category": "public-api",
          "endpoint": {
            "id": "public_1",
            "port": 4101
          },
          "command-deps": [
            {
              "command": "call-user-api",
              "proxies": ["auth_proxy", "audit_proxy"]
            }
          ]
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
    config "github.com/noPerfection/runtime/config"
)

a, err := config.Load("app.json")
if err != nil {
    panic(err)
}

svc, err := a.GetService("public_api")
if err != nil {
    panic(err)
}

updated := svc
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

Each `command-deps` entry must name a `command` and at least one routing target: `proxies` and/or `extensions`. A command without dependencies is invalid.

Each entry in `proxies` or `extensions` is a `DepTarget`: either a service name string (reference into `services`) or an inline service object. `config.Load` calls `Normalize()` to register inline services and verify references.

```json
"proxies": [
  "auth_proxy",
  {
    "type": "Proxy",
    "name": "inline_audit",
    "handlers": [ ... ]
  }
]
```

Proxy chains are declared in handler `command-deps` metadata. Terminal services that only receive routed traffic do not need `command-deps`.

## Packages removed from this module

Previous versions included a dev runtime layer (`engine`, `handler`, `client`, `watch`) for serving config over noPerfection sockets. That runtime API has been removed. Consumers should load JSON with `config.Load` and save it with `NoPerfection.Save`.

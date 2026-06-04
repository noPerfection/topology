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

Each `handler-deps` or `command-deps` entry must have a `name` and at least one routing target: `proxies` and/or `extensions`. In `handler-deps`, `name` is the handler category. In `command-deps`, `name` is the command name.

Each entry in `proxies` or `extensions` is a `DepTarget`: either a service name string (reference into `services`), an inline service object, or an inline proxy object. `config.Load` calls `Normalize()` to register inline targets and verify references. The JSON stays compact: a target is one value, not an object with separate `ref`/`service`/`proxy` keys.

```json
"proxies": [
  "auth_proxy",
  {
    "type": "Proxy",
    "name": "inline_audit",
    "handlers": [
      {
        "type": "Replier",
        "category": "audit",
        "endpoint": {"id": "audit_1", "port": 4301},
        "outbounds": ["audit_sink"]
      }
    ]
  }
]
```

Proxy chains can be declared in service-level `handler-deps` metadata or per-handler `command-deps` metadata. Terminal services that only receive routed traffic do not need either section.

## Packages removed from this module

Previous versions included a dev topology layer (`engine`, `handler`, `client`, `watch`) for serving config over noPerfection sockets. That topology API has been removed. Consumers should load JSON with `config.Load` and save it with `NoPerfection.Save`.

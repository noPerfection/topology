# noPerfection Configuration

This package defines the static JSON configuration model for a noPerfection topology. It contains Go types, JSON marshal/unmarshal helpers, `Load`, `NoPerfection.Save`, and validation. It does not run a topology server or client.

## Topology Model

The config describes a topology as a graph.

- A `Service` is a node in the graph.
- A `Handler` is an entry point on that node.
- A `DepService` is an edge declaration: it says which other nodes a handler or command may call.
- A `ServicePointer` is the target of that edge, either by reference or as an inline nested service.

This separation keeps identity, transport, and routing independent:

- `Service` owns deployment metadata such as `module-url` and `start-command`.
- `Handler` owns endpoint metadata such as protocol type, category, and address.
- `DepService` owns dependency intent without embedding the target service into every caller.
- `ServicePointer` lets the same dependency point to a shared service or define an inline private service.

```
NoPerfection
└── services[]: Service
    ├── handler-deps[]: DepService          (service-wide routing by handler category)
    └── handlers[]: Handler
        ├── IndependentHandler              (single endpoint)
        └── ProxyHandler                    (endpoint + outbound routing)
            └── command-deps[]: DepService  (per-command routing)
                ├── proxies[]: ServicePointer
                └── extensions[]: ServicePointer
```

See [examples/app-proxy-chain.json](examples/app-proxy-chain.json) for a complete graph.

## Root Config

`NoPerfection` is the JSON document root. It contains the services that can be resolved by name and validated as one topology.

```json
{
  "services": [
    {
      "type": "Independent",
      "name": "public_api",
      "handlers": []
    }
  ]
}
```

`Load` reads this document and validates the whole graph. `Save` writes the same structure back to disk.

## Services

A `Service` is the unit that gets registered, looked up, started, and referenced by dependencies. Its `name` is the stable identity used by refs such as `"auth_proxy"` or `"auth_proxy/main"`.

The `type` describes the role the service plays in routing:

| `type` | Role |
|--------|------|
| `Independent` | A normal service that handles its own traffic. |
| `Proxy` | A service that can forward commands to other services. May contain both plain handlers and proxy handlers. |
| `Extension` | A service used as an extension target in dependency routing. |

Bootstrap fields live on the service, not on each handler, because starting or loading code is a property of the service process/module:

- `module-url` — how to load an inproc handler.
- `start-command` — how to start an IPC handler.

`handler-deps` belongs here when routing is tied to a handler category regardless of command. `parameters` is free-form service metadata and is intentionally not interpreted by this package.

## Handlers

A service may expose multiple handlers because one service can have several entry points: public API, internal API, manager endpoint, publisher, pair socket, and so on. The `category` is the stable label used to choose between those entry points.

In JSON, each `handlers[]` entry is either an `IndependentHandler` or a `ProxyHandler`. The unmarshaller chooses `ProxyHandler` when proxy routing fields such as `outbounds`, `routes`, or `forward` are present.

`IndependentHandler` is the base shape: protocol type, category, endpoint, and optional command-level dependencies. It represents an endpoint that receives or serves traffic.

`ProxyHandler` embeds the same base shape and adds outbound routing:

- `outbounds` — inline `Service` definitions where proxied traffic may be sent.
- `routes` — optional whitelist of command routes this handler accepts.
- `forward` — optional map from a route to a specific outbound.

Notice: `outbounds` are inline services, not refs. Referenced services may have their own proxies, so following a ref from an outbound would require traversing another topology path to find the final endpoint.

A `Proxy` service must use `ProxyHandler` for every handler in `handlers`.

## Dependencies

Dependencies are declared separately from services so callers describe intent, while targets remain reusable and independently validated.

`DepService` is used in two scopes. In `handler-deps`, `name` is a handler category on the service. In `command-deps`, `name` is a command handled by the parent handler.

| Declared in | `name` means |
|-------------|--------------|
| `handler-deps` | handler category on this service |
| `command-deps` | command name handled by the parent handler |

Each dependency lists proxy targets, extension targets, or both. Proxies are for routed calls through another service. Extensions are for extension-style targets.

`ServicePointer` is the target value in `proxies` and `extensions`. It is either:

- a **ref** to an existing service in `services`, e.g. `"auth_proxy"` or `"auth_proxy/main"`.
- an **inline service** object defined directly in the JSON.

Refs make shared dependencies explicit. Inline services keep private or nested dependencies close to the caller without requiring a top-level service entry.

## Ref Format

Refs point to services declared in the root `services` list.

- `service_name`: references a service.
- `service_name/handler_category`: references one handler category on a service.

Invalid refs include empty strings, trailing slashes, empty service names, and empty path segments such as `"auth_proxy//main"`.

## Endpoint Bootstrap Rules

Endpoint `port` defaults to `0` when omitted.

- `port: 0` with an id starting with `tmp` is IPC and requires `start-command`.
- `port: 0` with any other id is inproc and requires `module-url`.
- non-zero ports do not require bootstrap metadata.
- manager handlers (`category: "manager"`) are ignored when deciding service bootstrap requirements.

## Validation

`Load` calls `ValidateTopology`.

Validation checks service names, service types, handler types, handler categories, endpoint ids, dependency target shape, inline service definitions, and ref existence. If a ref includes a handler category, the referenced service must contain that category.

For `ProxyHandler`, validation also checks outbound targets and verifies that `forward` entries refer to declared routes and outbounds.

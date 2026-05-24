# Dev Lib

`dev-lib` provides the development context for SDS services.

The context owns the local service configuration and exposes an internal runtime
client for starting, stopping, adding, updating, and removing dependency services
during development.

## Current Model

Dependency services are configured with `config-lib` and started from each
service's `StartCommand`.

Use the current module version from Git:

```sh
go get github.com/sds-framework/dev-lib@latest
```

This version requires `github.com/sds-framework/config-lib` with `config.Socket`,
`config.Service`, `config.Handler`, and `SdsService.Save()` support.

## Context API

Create a context from a config file and the socket that the internal runtime
handler should bind to. Then start the in-process runtime handler and use
`Runtime()` to control dependency services.

```go
package main

import (
	dev "github.com/sds-framework/dev-lib"
	config "github.com/sds-framework/config-lib"
)

func main() {
	runtimeSocket := config.Socket{
		Id:   "dep_handler",
		Port: 0,
	}

	ctx, err := dev.New("service.json", runtimeSocket)
	if err != nil {
		panic(err)
	}

	if err := ctx.StartRuntimeHandler(); err != nil {
		panic(err)
	}

	runtimeClient := ctx.Runtime()
	_ = runtimeClient
}
```

The public context interface is intentionally small:

```go
type Interface interface {
	StartRuntimeHandler() error
	Runtime() runtime.ClientInterface
}
```

There is no public `CloseRuntimeHandler` or `IsHandlerRunning` API. The handler
is an internal in-process runtime detail; users normally control services via
`StartService` and `StopService`.

When `New` loads the config, it ensures there is exactly one independent service.
If none exists, it creates a default independent service for the internal runtime.
If one exists, it ensures that service has the internal runtime handler category
and socket. More than one independent service is an error.

## Runtime Client

`ctx.Runtime()` returns a `runtime.ClientInterface`.

```go
type ClientInterface interface {
	Close() error
	Timeout(duration time.Duration)
	Attempt(attempt uint8)

	AddService(service config.Service) error
	SetService(service config.Service) error
	RemoveService(serviceName string) error
	StartService(serviceName string, parent *clientConfig.Client) (string, error)
	StopService(serviceName string) error
	IsServiceRunning(serviceName string) (bool, error)
}
```

Example:

```go
id, err := ctx.Runtime().StartService("database", parentClient)
if err != nil {
	panic(err)
}

running, err := ctx.Runtime().IsServiceRunning("database")
if err != nil {
	panic(err)
}

if running {
	if err := ctx.Runtime().StopService("database"); err != nil {
		panic(err)
	}
}

_ = id
```

`StartService` returns the generated runtime id for the started service, for
example `database1`.

## Runtime Package

The runtime package now contains the service runtime, runtime handler, and
runtime client.

Constructors:

```go
rt := runtime.New(cfg)
handler, err := runtime.NewHandler(cfg, runtimeSocket)
client, err := runtime.NewClient(runtimeSocket)
```

The old `dep_client` and `dep_handler` packages were folded into `runtime`.
Their generic `New()` constructors were renamed to avoid collisions:

- `dep_handler.New(...)` became `runtime.NewHandler(...)`
- `dep_client.New()` became `runtime.NewClient()`

## Service Management

Services are added, updated, and removed through config-backed runtime commands:

```go
service := config.Service{
	Type:         config.ExtensionType,
	Name:         "worker",
	StartCommand: "./worker",
	Handlers: []config.Handler{
		{
			Type:     config.ReplierType,
			Category: "manager",
			Socket: config.Socket{
				Id:   "worker-manager",
				Port: 6001,
			},
		},
	},
}

if err := ctx.Runtime().AddService(service); err != nil {
	panic(err)
}

service.StartCommand = "./worker --debug"
if err := ctx.Runtime().SetService(service); err != nil {
	panic(err)
}

if err := ctx.Runtime().RemoveService("worker"); err != nil {
	panic(err)
}
```

`AddService` refuses to add independent services and refuses to overwrite an
existing service. `SetService` updates an existing service. `RemoveService`
refuses to remove a service that is currently running.

## Service Requirements

Every service managed by `dev-lib` must have at least one handler that manages
the service itself. By convention this handler uses category `manager`.

The runtime uses the `manager` handler to:

- connect to the service
- send `heartbeat` requests for `IsServiceRunning`
- send the close command for `StopService`

Example service config:

```json
{
  "type": "Extension",
  "name": "worker",
  "start-command": "./worker",
  "handlers": [
    {
      "type": "Replier",
      "category": "manager",
      "socket": {
        "id": "worker-manager",
        "port": 6001
      }
    }
  ]
}
```

User-facing API handlers can be added beside the `manager` handler. The manager
handler is reserved for runtime lifecycle operations.

Independent services are special: there can be only one independent service in
the config, and it represents the service currently using the runtime. It cannot
be added through `AddService` or stopped through `StopService`.

## Handler Details

`StartRuntimeHandler` starts an in-process handler that exposes runtime commands
over SDS handler sockets:

- `add-service`
- `set-service`
- `remove-service`
- `start-service`
- `stop-service`
- `is-service-running`

Applications usually do not interact with this handler directly. They use
`ctx.Runtime()` instead.

## Tests

Run the root package tests:

```sh
go test .
```

Runtime tests compile, but tests that start sample binaries require local test
fixtures for those binaries. The old `_test_services` submodules have been
removed.

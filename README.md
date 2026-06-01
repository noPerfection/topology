# Context

`context` is a Go library for applications built on the SDS framework.

`context` owns your app's SDS service configuration and exposes a runtime client
for starting, stopping, adding, updating, and removing dependency services while
your app is running. You can manage the services both programmatically, by
simply calling the client, or you can call it by cli.

The `context` uses the simples form of management. It uses commands and calls `os.Exec`
as if its a bash script.

If you want to manage your app in another environment, lets say in Kubernetes or Docker,
then use `context.Interface` and simpply replace this module with yours.

The `context.Interface` from `interface.go` in the root of this module:

```go
type Interface interface {
	StartRuntimeHandler() error
	Runtime() runtime.ClientInterface
}
```

That keeps most of your app independent from the concrete `Context` struct while
still giving it access to service lifecycle operations through `ctx.Runtime()`.

## Install

```sh
go get github.com/noPerfection/context@latest
```

## Setup

Assume you are implementing an SDS service. This is close to how an app built
from `github.com/sds-framework/service` would wire its startup.

First, load the context module and config types:

```go
package main

import (
	sdscontext "github.com/noPerfection/context"
	config "github.com/noPerfection/context/config"
)
```

Second, choose the configuration file name. The file does not have to exist yet.
It is where `context` will load and store your app's SDS service configuration.

Config does two things: **service metadata** and **runtime wiring**.

```go
const configPath = "service.json"
```

Finally, set up context, launch its runtime handler, and call `ctx.Runtime()`
from your service code:

```go
func main() {
	runtimeSocket := config.Socket{
		Id:   "runtime",
		Port: 0,
	}

	ctx, err := sdscontext.New(configPath, runtimeSocket)
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

The `context/config` import is only needed here because `New` currently accepts
a `config.Socket`. You do not need to call `config.Load` yourself.

Use the interface in the rest of your app:

```go
func runService(ctx sdscontext.Interface) error {
	runtimeClient := ctx.Runtime()
	_ = runtimeClient
	return nil
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
	StartService(serviceName string, parent *runtime.ParentClient) (string, error)
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

The `runtime` package contains the service runtime, runtime handler, and
runtime client used by `Context`.

Constructors:

```go
rt := runtime.New(cfg)
handler, err := runtime.NewHandler(cfg, runtimeSocket)
client, err := runtime.NewClient(runtimeSocket)
```

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

Every service managed by `context` must have at least one handler that manages
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
fixtures for those binaries under `_test_services`.

# Dev Lib

`dev-lib` provides the development context for SDS services.

The context owns the local service configuration and exposes a runtime client for
starting, stopping, adding, and removing dependency services during development.

## Current Model

Dependency services are configured in `config-lib` service config and started
from each service's `StartCommand`.

## Context API

Create a context from a config file, start the in-process runtime handler, then
use `Runtime()` to control dependency services.

```go
package main

import (
	dev "github.com/sds-framework/dev-lib"
)

func main() {
	ctx, err := dev.New("service.json")
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

## Runtime Client

`ctx.Runtime()` returns a `runtime.ClientInterface`.

```go
type ClientInterface interface {
	Close() error
	Timeout(duration time.Duration)
	Attempt(attempt uint8)

	AddService(service config.Service) error
	RemoveService(serviceName string) error
	StartService(serviceName string, parent *clientConfig.Client) (string, error)
	StopService(depClient *clientConfig.Client) error
	IsServiceRunning(depClient *clientConfig.Client) (bool, error)
}
```

Example:

```go
id, err := ctx.Runtime().StartService("database", parentClient)
if err != nil {
	panic(err)
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
handler, err := runtime.NewHandler(cfg)
client, err := runtime.NewClient()
```

The old `dep_client` and `dep_handler` packages were folded into `runtime`.
Their generic `New()` constructors were renamed to avoid collisions:

- `dep_handler.New(...)` became `runtime.NewHandler(...)`
- `dep_client.New()` became `runtime.NewClient()`

## Service Management

Services are added and removed through config-backed runtime commands:

```go
service := config.Service{
	Name:         "worker",
	StartCommand: "./worker",
}

if err := ctx.Runtime().AddService(service); err != nil {
	panic(err)
}

if err := ctx.Runtime().RemoveService("worker"); err != nil {
	panic(err)
}
```

`RemoveService` refuses to remove a service that is currently running.

## Handler Details

`StartRuntimeHandler` starts an in-process handler that exposes runtime commands
over SDS handler sockets:

- `add-service`
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

# Topology

`topology` provides a dependency services manager for noPerfection microservices.
With `topology`, noPerfection services can manage their dependencies.

Since its asynchronous and lives on another thread to not break service's own code, 
the runtime is decoupled into handler and a client.

## Install
Requires zmq library C library. Go code running or building must be then done using C enabling.

```sh
go get github.com/noPerfection/topology@latest
```

## Tutorial
First we need to start the runtime handler

```go
import "github.com/noPerfection/topology"
import "github.com/noPerfection/topology/config"
import "github.com/noPerfection/protocol/message"

//.. rest of code
runtimeEndpoint := message.NewEndpoint("runtime", 0)

handler, _ := topology.NewHandler("service.json", runtimeEndpoint)

// Any handler's functions.

// Register any additional handler routes or setup before starting, if needed.
if err := handler.Start(); err != nil {
	panic(err)
}

```
That's it. Runtime is running remotely.
Second, we need to interact with it from the code:

```go
	// Now interact with the runtime manager through a runtime client.
	runtimeClient, _ := topology.NewClient(runtimeEndpoint)
	defer runtimeClient.Close()

	running, err := runtimeClient.IsServiceRunning("database")
	if err != nil {
		panic(err)
	}

	_ = running
}
```

## Runtime Handler

`topology.NewHandler(configPath, runtimeEndpoint)` returns a handler that serves runtime commands over noPerfection protocol sockets. The handler loads `configPath` using `config.Load`, saves any runtime bootstrap changes, and uses `runtimeEndpoint` as its command endpoint.

The handler exposes these commands internally:

- `add-service`
- `set-service`
- `remove-service`
- `start-service`
- `stop-service`
- `is-service-running`

Applications usually do not send these commands directly. Use `topology.NewClient(runtimeEndpoint)` instead.

Before `Start()` is called, the returned handler also implements `topology.RuntimeInterface`. This lets setup code manipulate the runtime configuration directly:

```go
handler, _ := topology.NewHandler("service.json", runtimeEndpoint)

if err := handler.AddService(config.InlineTarget(service)); err != nil {
	panic(err)
}

if err := handler.Start(); err != nil {
	panic(err)
}
```

After `Start()` succeeds, direct runtime methods on the handler are unavailable and return an error. Use `topology.NewClient(runtimeEndpoint)` for `AddService`, `SetService`, `RemoveService`, `StartService`, `StopService`, and `IsServiceRunning` after launch.

## Runtime Client API

`topology.NewClient(runtimeEndpoint)` returns a `*topology.Client`. Configure request behavior with:

```go
runtimeClient.Timeout(5 * time.Second)
runtimeClient.Attempt(1)
```

Available client methods:

```go
type ClientInterface interface {
	Close() error
	Timeout(duration time.Duration)
	Attempt(attempt uint8)

	AddService(target config.DepTarget) error
	SetService(service config.Service) error
	RemoveService(serviceName string) error
	StartService(serviceName string, parent *topology.ParentClient) (string, error)
	StopService(serviceName string) error
	IsServiceRunning(serviceName string) (bool, error)
}
```

### Add or Update Services

```go
service := config.Service{
	Type:         config.ExtensionType,
	Name:         "worker",
	StartCommand: "./worker",
	Handlers: []config.Handler{
		{
			Type:     config.ReplierType,
			Category: topology.ManagerHandlerCategory,
			Endpoint: message.NewEndpoint("worker-manager", 6001),
		},
	},
}

if err := runtimeClient.AddService(config.InlineTarget(service)); err != nil {
	panic(err)
}

service.StartCommand = "./worker --debug"
if err := runtimeClient.SetService(service); err != nil {
	panic(err)
}
```

### Start, Check, and Stop Services

```go
parent := &topology.ParentClient{
	ServiceUrl: "api",
	Id:         "api-manager",
	Port:       6000,
}

id, err := runtimeClient.StartService("worker", parent)
if err != nil {
	panic(err)
}

if running, err := runtimeClient.IsServiceRunning("worker"); err != nil {
	panic(err)
} else if running {
	if err := runtimeClient.StopService("worker"); err != nil {
		panic(err)
	}
}

_ = id
```

### Remove Services

```go
if err := runtimeClient.RemoveService("worker"); err != nil {
	panic(err)
}
```

`RemoveService` refuses to remove a service that is currently running. `AddService` refuses to add independent services and refuses to overwrite an existing service. `SetService` updates an existing service.

## Service Requirements

Every managed service must have a handler that manages the service itself. By convention, this handler uses the `manager` category.

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
      "endpoint": {
        "id": "worker-manager",
        "port": 6001
      }
    }
  ]
}
```

Independent services are special: there can be only one independent service in the config, and it represents the service currently running the runtime handler. It cannot be added through `AddService` or stopped through `StopService`.

## Tests

Run the tests:

```sh
go test ./...
```

Runtime tests compile on a fresh checkout. Tests that start sample binaries require local fixtures under `_test_services`.

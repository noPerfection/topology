# Runtime

`runtime` provides a dependency services manager for noPerfection microservices.
With the `runtime` noPerfection services can manage its dependencies.

Since its asynchronous and lives on another thread to not break service's own code, 
the runtime is decoupled into handler and a client.

## Install
Requires zmq library C library. Go code running or building must be then done using C enabling.

```sh
go get github.com/noPerfection/runtime@latest
```

## Tutorial
First we need to start the runtime handler

```go
import "github.com/noPerfection/runtime"
import "github.com/noPerfection/runtime/config"

//.. rest of code
runtimeSocket := config.Socket{
	Id:   "runtime",
	Port: 0,
}

handler, _ := runtime.NewHandler("service.json", runtimeSocket)

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
	runtimeClient, _ := runtime.NewClient(runtimeSocket)
	defer runtimeClient.Close()

	running, err := runtimeClient.IsServiceRunning("database")
	if err != nil {
		panic(err)
	}

	_ = running
}
```

## Runtime Handler

`runtime.NewHandler(configPath, runtimeSocket)` returns a handler that serves runtime commands over noPerfection protocol sockets. The handler loads `configPath` using `config.Load`, saves any runtime bootstrap changes, and uses `runtimeSocket` as its command endpoint.

The handler exposes these commands internally:

- `add-service`
- `set-service`
- `remove-service`
- `start-service`
- `stop-service`
- `is-service-running`

Applications usually do not send these commands directly. Use `runtime.NewClient(runtimeSocket)` instead.

## Runtime Client API

`runtime.NewClient(runtimeSocket)` returns a `*runtime.Client`. Configure request behavior with:

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
	StartService(serviceName string, parent *runtime.ParentClient) (string, error)
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
			Category: runtime.ManagerHandlerCategory,
			Socket: config.Socket{
				Id:   "worker-manager",
				Port: 6001,
			},
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
parent := &runtime.ParentClient{
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
      "socket": {
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

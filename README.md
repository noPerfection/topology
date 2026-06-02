# Topology

`topology` provides a dependency topology manager for noPerfection microservices.
With `topology`, noPerfection services can manage their dependencies.

Topology is an in-process (`inproc`) handler: it lives in the same process as
the service, but runs asynchronously on a different goroutine/thread so it does
not block the service's own code. The handler and client use a predefined
endpoint at `inproc://<TopologyHandlerCategory>`, where `TopologyHandlerCategory`
is the package constant used to build the endpoint internally.

## Install
Requires zmq library C library. Go code running or building must be then done using C enabling.

```sh
go get github.com/noPerfection/topology@latest
```

## Tutorial
First we need to start the topology handler

```go
import "github.com/noPerfection/topology"

//.. rest of code
handler, _ := topology.NewHandler("service.json")

// Any handler's functions.

// Register any additional handler routes or setup before starting, if needed.
if err := handler.Start(); err != nil {
	panic(err)
}

```
That's it. Topology is running in-process.
Second, we need to interact with it from the code:

```go
	// Now interact with the topology manager through a topology client.
	topologyClient, _ := topology.NewClient()
	defer topologyClient.Close()

	running, err := topologyClient.IsServiceRunning("database")
	if err != nil {
		panic(err)
	}

	_ = running
}
```

## Topology Handler

`topology.NewHandler(configPath)` returns a handler that serves topology commands over an in-process noPerfection protocol socket. The handler loads `configPath` using `config.Load`, saves any topology bootstrap changes, and uses the predefined `inproc://<TopologyHandlerCategory>` command endpoint.

The handler exposes these commands internally:

- `add-service`
- `set-service`
- `remove-service`
- `start-service`
- `stop-service`
- `is-service-running`

Applications usually do not send these commands directly. Use `topology.NewClient()` instead.

Before `Start()` is called, the returned handler also implements `topology.TopologyInterface`. This lets setup code manipulate the topology configuration directly:

```go
handler, _ := topology.NewHandler("service.json")

if err := handler.AddService(config.InlineTarget(service)); err != nil {
	panic(err)
}

if err := handler.Start(); err != nil {
	panic(err)
}
```

After `Start()` succeeds, direct topology methods on the handler are unavailable and return an error. Use `topology.NewClient()` for `AddService`, `SetService`, `RemoveService`, `StartService`, `StopService`, and `IsServiceRunning` after launch.

## Topology Client API

`topology.NewClient()` returns a `*topology.Client`. It builds the same in-process endpoint internally with `message.NewEndpoint(TopologyHandlerCategory, 0)`, which resolves to `inproc://<TopologyHandlerCategory>`. Configure request behavior with:

```go
topologyClient.Timeout(5 * time.Second)
topologyClient.Attempt(1)
```

Available client methods:

```go
type NodeInterface interface {
	StopService(serviceName string) error
	StartService(serviceName string, optionalParent ...*topology.ParentClient) (string, error)
	IsServiceRunning(serviceName string) (bool, error)
}

type TopologyInterface interface {
	NodeInterface

	AddService(target config.DepTarget) error
	SetService(service config.Service) error
	RemoveService(serviceName string) error
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
			Category: topology.ServiceManagerCategory,
			Endpoint: message.NewEndpoint("worker-manager", 6001),
		},
	},
}

if err := topologyClient.AddService(config.InlineTarget(service)); err != nil {
	panic(err)
}

service.StartCommand = "./worker --debug"
if err := topologyClient.SetService(service); err != nil {
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

id, err := topologyClient.StartService("worker", parent)
if err != nil {
	panic(err)
}

if running, err := topologyClient.IsServiceRunning("worker"); err != nil {
	panic(err)
} else if running {
	if err := topologyClient.StopService("worker"); err != nil {
		panic(err)
	}
}

_ = id
```

### Remove Services

```go
if err := topologyClient.RemoveService("worker"); err != nil {
	panic(err)
}
```

`RemoveService` refuses to remove a service that is currently running. `AddService` refuses to add independent services and refuses to overwrite an existing service. `SetService` updates an existing service.

## Service Requirements

Every managed service must have a handler that manages the service itself. By convention, this handler uses the `manager` category.

The topology uses the `manager` handler to:

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

Independent services are special: there can be only one independent service in the config, and it represents the service currently running the topology handler. It cannot be added through `AddService` or stopped through `StopService`.

## Tests

Run the tests:

```sh
go test ./...
```

Topology tests compile on a fresh checkout. Tests that start sample binaries require local fixtures under `_test_services`.

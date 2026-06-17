// Package topology manages dependency service lifecycle for noPerfection services.
package topology

import (
	"fmt"
	"sync"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	clientSyncReplier "github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/control"
	"github.com/noPerfection/protocol/handler/replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology/config"
)

const (
	TopologyHandlerCategory = "service_topology" // handler category
	TopologySocketType        = handlerConfig.ReplierType
	IsRunning                 = "is-running"
	IsServiceRunning          = "is-service-running"
	StartService              = "start-service"
	StopService               = "stop-service"
	Service                   = "service"
	Services                  = "services"
	AddService                = "add-service"
	SetService                = "set-service"
	RemoveService             = "remove-service"
)

// Handler acts as the router from other app processes to the topology.
type Handler struct {
	handler  base.Interface    // Receive commands
	topology TopologyInterface // Route to the functions from topology
	started  bool
	mu       sync.Mutex
}

var _ TopologyInterface = (*Handler)(nil)

var topologyMutationMu sync.Mutex

// HandlerConfig returns the handler configuration for the topology endpoint.
// Then use it as the handler's config with the SetConfig method.
func HandlerConfig() *handlerConfig.Handler {
	return handlerConfig.New(
		TopologySocketType,
		TopologyHandlerCategory,
		TopologyHandlerCategory,
		0,
	)
}

// NewHandler loads app config, ensures the independent topology service entry exists,
// persists config when it changed, and returns a dependency topology handler.
func NewHandler(configPath string) (*Handler, error) {
	appConfig, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("config.Load('%s'): %w", configPath, err)
	}

	handler := replier.New()

	logger, err := log.New(TopologyHandlerCategory, true)
	if err != nil {
		return nil, fmt.Errorf("log.New('%s'): %w", TopologyHandlerCategory, err)
	}

	handler.SetConfig(HandlerConfig())
	err = handler.SetLogger(logger)
	if err != nil {
		return nil, fmt.Errorf("handler.SetLogger: %w", err)
	}

	return &Handler{
		topology: New(&appConfig),
		handler:  handler,
	}, nil
}

func (h *Handler) requireNotStarted() error {
	if h == nil {
		return fmt.Errorf("handler is nil")
	}
	if h.started {
		return fmt.Errorf("topology handler already started, use topology client")
	}
	if h.topology == nil {
		return fmt.Errorf("topology is nil")
	}
	return nil
}

func optionalParent(params datatype.KeyValue) []string {
	parent, err := params.StringValue("parent")
	if err != nil || parent == "" {
		return nil
	}
	return []string{parent}
}

// AddService registers a service in the topology configuration before the
// topology handler is started.
func (h *Handler) AddService(record config.Service, parent ...string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.AddService(record, parent...)
}

// SetService updates a service in the topology configuration before the topology
// handler is started.
func (h *Handler) SetService(record config.Service, parent ...string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.SetService(record, parent...)
}

// RemoveService removes a service from the topology configuration before the
// topology handler is started.
func (h *Handler) RemoveService(name string, parent ...string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.RemoveService(name, parent...)
}

// StartService starts a dependency service before the topology handler is
// started.
func (h *Handler) StartService(mushroomURL string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return "", err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.StartService(mushroomURL)
}

// IsServiceRunning checks a dependency service before the topology handler is
// started.
func (h *Handler) IsServiceRunning(mushroomURL string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return false, err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.IsServiceRunning(mushroomURL)
}

// StopService stops a dependency service before the topology handler is started.
func (h *Handler) StopService(mushroomURL string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.StopService(mushroomURL)
}

// Service returns a service configuration before the topology handler is started.
func (h *Handler) Service(mushroomURL string) (config.Service, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return config.Service{}, err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.Service(mushroomURL)
}

// Services returns service configurations before the topology handler is started.
func (h *Handler) Services() ([]config.Service, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return nil, err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.Services()
}

func (h *Handler) onIsRunning(req message.RequestInterface) message.ReplyInterface {
	return req.Ok(datatype.New().Set("running", true))
}

// onIsServiceRunning checks whether the dependency is running or not.
// Requires 'service' — a service name or dereference Mushroom URL.
func (h *Handler) onIsServiceRunning(req message.RequestInterface) message.ReplyInterface {
	mushroomURL, err := req.RouteParameters().StringValue(Service)
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	running, err := h.topology.IsServiceRunning(mushroomURL)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.IsServiceRunning: %v", err))
	}

	params := datatype.New().Set("running", running)
	return req.Ok(params)
}

// onStartService starts the dependency service.
// Requires 'service' — a service name or dereference Mushroom URL.
func (h *Handler) onStartService(req message.RequestInterface) message.ReplyInterface {
	mushroomURL, err := req.RouteParameters().StringValue(Service)
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	id, err := h.topology.StartService(mushroomURL)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.StartService(%q): %v", mushroomURL, err))
	}

	return req.Ok(datatype.New().Set("id", id))
}

// onAddService registers a service in the topology configuration.
// Requires 'service' as a service object.
func (h *Handler) onAddService(req message.RequestInterface) message.ReplyInterface {
	kv, err := req.RouteParameters().NestedValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetKeyValue('service'): %v", err))
	}

	var record config.Service
	if err := kv.Interface(&record); err != nil {
		return req.Fail(fmt.Sprintf("kv.Interface('config.Service'): %v", err))
	}

	if err := h.topology.AddService(record, optionalParent(req.RouteParameters())...); err != nil {
		return req.Fail(fmt.Sprintf("h.topology.AddService('%s'): %v", record.Name, err))
	}

	return req.Ok(datatype.New())
}

// onSetService updates a service in the topology configuration.
// Requires 'service' of the config.Service type.
func (h *Handler) onSetService(req message.RequestInterface) message.ReplyInterface {
	kv, err := req.RouteParameters().NestedValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetKeyValue('service'): %v", err))
	}

	var record config.Service
	err = kv.Interface(&record)
	if err != nil {
		return req.Fail(fmt.Sprintf("kv.Interface: %v", err))
	}

	err = h.topology.SetService(record, optionalParent(req.RouteParameters())...)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.SetService('%s'): %v", record.Name, err))
	}

	return req.Ok(datatype.New())
}

// onRemoveService removes a service from the topology configuration.
// Requires 'service' string parameter with the service name.
func (h *Handler) onRemoveService(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	err = h.topology.RemoveService(serviceName, optionalParent(req.RouteParameters())...)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.RemoveService('%s'): %v", serviceName, err))
	}

	return req.Ok(datatype.New())
}

// onStopService stops the dependency.
// Requires 'service' — a service name or dereference Mushroom URL.
func (h *Handler) onStopService(req message.RequestInterface) message.ReplyInterface {
	mushroomURL, err := req.RouteParameters().StringValue(Service)
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	err = h.topology.StopService(mushroomURL)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.StopService: %v", err))
	}

	return req.Ok(datatype.New())
}

// onService returns the configuration for a service.
// Requires 'service' — a service name or dereference Mushroom URL.
func (h *Handler) onService(req message.RequestInterface) message.ReplyInterface {
	mushroomURL, err := req.RouteParameters().StringValue(Service)
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	record, err := h.topology.Service(mushroomURL)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.Service(%q): %v", mushroomURL, err))
	}

	return req.Ok(datatype.New().Set("service", record))
}

// onServices returns the configuration for all services.
func (h *Handler) onServices(req message.RequestInterface) message.ReplyInterface {
	records, err := h.topology.Services()
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.Services: %v", err))
	}

	return req.Ok(datatype.New().Set("services", records))
}

// Start starts the dependency handler with the available operations.
func (h *Handler) Start() error {
	if h == nil {
		return fmt.Errorf("handler is nil")
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.started {
		return nil
	}
	if h.isTopologyAlreadyRunning() {
		h.started = true
		return nil
	}
	if h.restartExistingTopologyHandler() {
		h.started = true
		return nil
	}

	if err := h.handler.Route(IsRunning, h.onIsRunning); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", IsRunning, err)
	}
	if err := h.handler.Route(IsServiceRunning, h.onIsServiceRunning); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", IsServiceRunning, err)
	}
	if err := h.handler.Route(StartService, h.onStartService); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", StartService, err)
	}
	if err := h.handler.Route(StopService, h.onStopService); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", StopService, err)
	}
	if err := h.handler.Route(Service, h.onService); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", Service, err)
	}
	if err := h.handler.Route(Services, h.onServices); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", Services, err)
	}
	if err := h.handler.Route(AddService, h.onAddService); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", AddService, err)
	}
	if err := h.handler.Route(SetService, h.onSetService); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", SetService, err)
	}
	if err := h.handler.Route(RemoveService, h.onRemoveService); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", RemoveService, err)
	}

	if err := h.handler.Start(); err != nil {
		return err
	}
	h.started = true
	return nil
}

func (h *Handler) isTopologyAlreadyRunning() bool {
	client, err := NewClient()
	if err != nil {
		return false
	}
	defer client.Close()

	client.Timeout(50 * time.Millisecond)
	client.Attempt(2)
	running, err := client.IsRunning()
	return err == nil && running
}

func (h *Handler) restartExistingTopologyHandler() bool {
	controlConfig := control.CreateInternalConfig(HandlerConfig())
	client, err := clientSyncReplier.NewClient(controlConfig.Id, controlConfig.Port)
	if err != nil {
		return false
	}
	defer client.Close()

	client.Timeout(50 * time.Millisecond)
	client.Attempt(2)
	reply, err := client.Request(&message.Request{
		Command:    control.HandlerStart,
		Parameters: datatype.New(),
	})
	return err == nil && reply.IsOK()
}

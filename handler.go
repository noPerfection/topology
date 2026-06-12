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
	TopologyHandlerCategory   = "service_topology" // handler category
	TopologySocketType        = handlerConfig.ReplierType
	IsRunning                 = "is-running"
	IsServiceRunning          = "is-service-running"
	IsServiceRunningByManager = "is-service-running-by-manager"
	StartService              = "start-service"
	StartServiceByConfig      = "start-service-by-config"
	StopService               = "stop-service"
	StopServiceByManager      = "stop-service-by-manager"
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

// AddService registers a service in the topology configuration before the
// topology handler is started.
func (h *Handler) AddService(record config.Service) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.AddService(record)
}

// SetService updates a service in the topology configuration before the topology
// handler is started.
func (h *Handler) SetService(record config.Service) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.SetService(record)
}

// RemoveService removes a service from the topology configuration before the
// topology handler is started.
func (h *Handler) RemoveService(serviceName string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.RemoveService(serviceName)
}

// StartService starts a dependency service before the topology handler is
// started.
func (h *Handler) StartService(serviceName string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return "", err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.StartService(serviceName)
}

// StartServiceByConfig registers and starts a dependency service before the
// topology handler is started.
func (h *Handler) StartServiceByConfig(record config.Service) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return "", err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.StartServiceByConfig(record)
}

// IsServiceRunning checks a dependency service before the topology handler is
// started.
func (h *Handler) IsServiceRunning(serviceName string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return false, err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.IsServiceRunning(serviceName)
}

// IsServiceRunningByManager checks a dependency service manager before the
// topology handler is started.
func (h *Handler) IsServiceRunningByManager(serviceName string, handler config.IndependentHandler) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return false, err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.IsServiceRunningByManager(serviceName, handler)
}

// StopService stops a dependency service before the topology handler is started.
func (h *Handler) StopService(serviceName string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.StopService(serviceName)
}

// StopServiceByManager stops a dependency service manager before the topology
// handler is started.
func (h *Handler) StopServiceByManager(serviceName string, handler config.IndependentHandler) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.StopServiceByManager(serviceName, handler)
}

// Service returns a service configuration before the topology handler is started.
func (h *Handler) Service(serviceName string) (config.Service, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.requireNotStarted(); err != nil {
		return config.Service{}, err
	}
	topologyMutationMu.Lock()
	defer topologyMutationMu.Unlock()

	return h.topology.Service(serviceName)
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
// Requires 'service' string parameter with the service name.
func (h *Handler) onIsServiceRunning(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	running, err := h.topology.IsServiceRunning(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.IsServiceRunning: %v", err))
	}

	params := datatype.New().Set("running", running)
	return req.Ok(params)
}

// onIsServiceRunningByManager checks whether the dependency is running by its
// manager handler.
// Requires 'service' string parameter and 'handler' as a config.IndependentHandler object.
func (h *Handler) onIsServiceRunningByManager(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	kv, err := req.RouteParameters().NestedValue("handler")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetKeyValue('handler'): %v", err))
	}

	var handler config.IndependentHandler
	if err := kv.Interface(&handler); err != nil {
		return req.Fail(fmt.Sprintf("kv.Interface('config.IndependentHandler'): %v", err))
	}

	running, err := h.topology.IsServiceRunningByManager(serviceName, handler)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.IsServiceRunningByManager: %v", err))
	}

	return req.Ok(datatype.New().Set("running", running))
}

// onStartService starts the dependency service.
// Requires:
//   - 'service' string parameter.
//
// Returns nothing.
// todo make it publish the result through publisher, so user won't wait for the result.
func (h *Handler) onStartService(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		serviceName, err = req.RouteParameters().StringValue("url")
		if err != nil {
			return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
		}
	}

	id, err := h.topology.StartService(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.StartService(service: '%s'): %v", serviceName, err))
	}

	return req.Ok(datatype.New().Set("id", id))
}

// onStartServiceByConfig registers and starts the dependency service.
// Requires 'service' as a service object.
func (h *Handler) onStartServiceByConfig(req message.RequestInterface) message.ReplyInterface {
	kv, err := req.RouteParameters().NestedValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetKeyValue('service'): %v", err))
	}

	var record config.Service
	if err := kv.Interface(&record); err != nil {
		return req.Fail(fmt.Sprintf("kv.Interface('config.Service'): %v", err))
	}

	id, err := h.topology.StartServiceByConfig(record)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.StartServiceByConfig('%s'): %v", record.Name, err))
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

	if err := h.topology.AddService(record); err != nil {
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

	err = h.topology.SetService(record)
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

	err = h.topology.RemoveService(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.RemoveService('%s'): %v", serviceName, err))
	}

	return req.Ok(datatype.New())
}

// onStopService stops the dependency.
// Requires 'service' string parameter with the service name.
func (h *Handler) onStopService(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	err = h.topology.StopService(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.StopService: %v", err))
	}

	return req.Ok(datatype.New())
}

// onStopServiceByManager stops the dependency by its manager handler.
// Requires 'service' string parameter and 'handler' as a config.IndependentHandler object.
func (h *Handler) onStopServiceByManager(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	kv, err := req.RouteParameters().NestedValue("handler")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetKeyValue('handler'): %v", err))
	}

	var handler config.IndependentHandler
	if err := kv.Interface(&handler); err != nil {
		return req.Fail(fmt.Sprintf("kv.Interface('config.IndependentHandler'): %v", err))
	}

	if err := h.topology.StopServiceByManager(serviceName, handler); err != nil {
		return req.Fail(fmt.Sprintf("h.topology.StopServiceByManager: %v", err))
	}

	return req.Ok(datatype.New())
}

// onService returns the configuration for a service.
func (h *Handler) onService(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	record, err := h.topology.Service(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.Service('%s'): %v", serviceName, err))
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
	if err := h.handler.Route(IsServiceRunningByManager, h.onIsServiceRunningByManager); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", IsServiceRunningByManager, err)
	}
	if err := h.handler.Route(StartService, h.onStartService); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", StartService, err)
	}
	if err := h.handler.Route(StartServiceByConfig, h.onStartServiceByConfig); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", StartServiceByConfig, err)
	}
	if err := h.handler.Route(StopService, h.onStopService); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", StopService, err)
	}
	if err := h.handler.Route(StopServiceByManager, h.onStopServiceByManager); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", StopServiceByManager, err)
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

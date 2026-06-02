// Package topology manages dependency service lifecycle for noPerfection services.
package topology

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology/config"
)

const (
	TopologyHandlerCategory = "service_topology" // handler category
	TopologySocketType      = handlerConfig.ReplierType
	IsServiceRunning        = "is-service-running"
	StartService            = "start-service"
	StopService             = "stop-service"
	AddService              = "add-service"
	SetService              = "set-service"
	RemoveService           = "remove-service"
)

// Handler acts as the router from other app processes to the topology.
type Handler struct {
	handler  base.Interface    // Receive commands
	topology TopologyInterface // Route to the functions from topology
	started  bool
}

var _ TopologyInterface = (*Handler)(nil)

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
func (h *Handler) AddService(target config.DepTarget) error {
	if err := h.requireNotStarted(); err != nil {
		return err
	}
	return h.topology.AddService(target)
}

// SetService updates a service in the topology configuration before the topology
// handler is started.
func (h *Handler) SetService(service config.Service) error {
	if err := h.requireNotStarted(); err != nil {
		return err
	}
	return h.topology.SetService(service)
}

// RemoveService removes a service from the topology configuration before the
// topology handler is started.
func (h *Handler) RemoveService(serviceName string) error {
	if err := h.requireNotStarted(); err != nil {
		return err
	}
	return h.topology.RemoveService(serviceName)
}

// StartService starts a dependency service before the topology handler is
// started.
func (h *Handler) StartService(serviceName string, optionalParent ...*ParentClient) (string, error) {
	if err := h.requireNotStarted(); err != nil {
		return "", err
	}
	return h.topology.StartService(serviceName, optionalParent...)
}

// IsServiceRunning checks a dependency service before the topology handler is
// started.
func (h *Handler) IsServiceRunning(serviceName string) (bool, error) {
	if err := h.requireNotStarted(); err != nil {
		return false, err
	}
	return h.topology.IsServiceRunning(serviceName)
}

// StopService stops a dependency service before the topology handler is started.
func (h *Handler) StopService(serviceName string) error {
	if err := h.requireNotStarted(); err != nil {
		return err
	}
	return h.topology.StopService(serviceName)
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

// onStartService starts the dependency service.
// Requires:
//   - 'service' string parameter.
//
// Optional:
//   - 'parent' of the ParentClient type.
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

	var optionalParent []*ParentClient
	if kv, err := req.RouteParameters().NestedValue("parent"); err == nil {
		var parent ParentClient
		if err := kv.Interface(&parent); err != nil {
			return req.Fail(fmt.Sprintf("kv.Interface: %v", err))
		}
		optionalParent = append(optionalParent, &parent)
	}

	id, err := h.topology.StartService(serviceName, optionalParent...)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.StartService(service: '%s'): %v", serviceName, err))
	}

	return req.Ok(datatype.New().Set("id", id))
}

// onAddService registers a service target in the topology configuration.
// Requires 'service' as either a service name or inline config.Service object.
func (h *Handler) onAddService(req message.RequestInterface) message.ReplyInterface {
	kv, err := req.RouteParameters().NestedValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetKeyValue('service'): %v", err))
	}

	var target config.DepTarget
	err = kv.Interface(&target)
	if err != nil {
		return req.Fail(fmt.Sprintf("kv.Interface: %v", err))
	}

	err = h.topology.AddService(target)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.AddService('%s'): %v", target.Name(), err))
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

	var service config.Service
	err = kv.Interface(&service)
	if err != nil {
		return req.Fail(fmt.Sprintf("kv.Interface: %v", err))
	}

	err = h.topology.SetService(service)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.topology.SetService('%s'): %v", service.Name, err))
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

// Start starts the dependency handler with the available operations.
func (h *Handler) Start() error {
	if h == nil {
		return fmt.Errorf("handler is nil")
	}
	if h.started {
		return fmt.Errorf("topology handler already started")
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

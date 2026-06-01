// Package runtime manages dependency service lifecycle for noPerfection services.
package runtime

import (
	"fmt"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/runtime/config"
)

const (
	RuntimeHandlerCategory = "service_runtime" // handler category
	RuntimeSocketType      = handlerConfig.ReplierType
	IsServiceRunning       = "is-service-running"
	StartService           = "start-service"
	StopService            = "stop-service"
	AddService             = "add-service"
	SetService             = "set-service"
	RemoveService          = "remove-service"
)

// Handler acts as the router from other app processes to the runtime.
type Handler struct {
	handler base.Interface   // Receive commands
	runtime RuntimeInterface // Route to the functions from runtime
}

// HandlerConfig returns the handler configuration for the runtime endpoint.
// Then use it as the handler's config with the SetConfig method.
func HandlerConfig(runtimeEndpoint message.Endpoint) *handlerConfig.Handler {
	return handlerConfig.New(
		RuntimeSocketType,
		runtimeEndpoint.Id,
		RuntimeHandlerCategory,
		runtimeEndpoint.Port,
	)
}

// NewHandler loads app config, ensures the independent runtime service entry exists,
// persists config when it changed, and returns a dependency runtime handler.
func NewHandler(configPath string, runtimeEndpoint message.Endpoint) (*Handler, error) {
	appConfig, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("config.Load('%s'): %w", configPath, err)
	}

	// appConfigChanged, err := ensureIndependentRuntimeService(&appConfig, runtimeEndpoint)
	// if err != nil {
	// 	return nil, fmt.Errorf("ensureIndependentRuntimeService: %w", err)
	// }
	// if appConfigChanged {
	// 	if err := appConfig.Save(); err != nil {
	// 		return nil, fmt.Errorf("appConfig.Save: %w", err)
	// 	}
	// }

	handler := replier.New()

	logger, err := log.New(RuntimeHandlerCategory, true)
	if err != nil {
		return nil, fmt.Errorf("log.New('%s'): %w", RuntimeHandlerCategory, err)
	}

	handler.SetConfig(HandlerConfig(runtimeEndpoint))
	err = handler.SetLogger(logger)
	if err != nil {
		return nil, fmt.Errorf("handler.SetLogger: %w", err)
	}

	return &Handler{
		runtime: New(&appConfig),
		handler: handler,
	}, nil
}

// func ensureIndependentRuntimeService(appConfig *config.NoPerfection, runtimeEndpoint message.Endpoint) (bool, error) {
// 	independentCount := appConfig.CountByType(config.IndependentType)
// 	if independentCount > 1 {
// 		return false, fmt.Errorf("only one independent service can be configured")
// 	}

// 	runtimeHandler := config.Handler{
// 		Type:     config.HandlerType(RuntimeSocketType),
// 		Category: RuntimeHandlerCategory,
// 		Endpoint: runtimeEndpoint,
// 	}

// 	if independentCount == 0 {
// 		err := appConfig.SetService(config.Service{
// 			Type:     config.IndependentType,
// 			Name:     RuntimeHandlerCategory,
// 			Handlers: []config.Handler{runtimeHandler},
// 		})
// 		if err != nil {
// 			return false, fmt.Errorf("appConfig.SetService: %w", err)
// 		}

// 		return true, nil
// 	}

// 	independentService, err := appConfig.GetByType(config.IndependentType)
// 	if err != nil {
// 		return false, fmt.Errorf("appConfig.GetByType('%s'): %w", config.IndependentType, err)
// 	}

// 	handler, err := independentService.HandlerByCategory(RuntimeHandlerCategory)
// 	if err == nil {
// 		if handler.Endpoint.Id == runtimeEndpoint.Id && handler.Endpoint.Port == runtimeEndpoint.Port {
// 			return false, nil
// 		}

// 		handler.Endpoint = runtimeEndpoint
// 		independentService.SetHandler(handler)
// 		return true, nil
// 	}

// 	independentService.Handlers = append(independentService.Handlers, runtimeHandler)
// 	return true, nil
// }

// onIsServiceRunning checks whether the dependency is running or not.
// Requires 'service' string parameter with the service name.
func (h *Handler) onIsServiceRunning(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	running, err := h.runtime.IsServiceRunning(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.runtime.IsServiceRunning: %v", err))
	}

	params := datatype.New().Set("running", running)
	return req.Ok(params)
}

// onStartService starts the dependency service.
// Requires:
//   - 'service' string parameter,
//   - 'parent' of the ParentClient type.
//
// Returns nothing.
// todo make it publish the result through publisher, so user won't wait for the result.
func (h *Handler) onStartService(req message.RequestInterface) message.ReplyInterface {
	kv, err := req.RouteParameters().NestedValue("parent")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetKeyValue('parent'): %v", err))
	}

	var parent ParentClient
	err = kv.Interface(&parent)
	if err != nil {
		return req.Fail(fmt.Sprintf("kv.Interface: %v", err))
	}

	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		serviceName, err = req.RouteParameters().StringValue("url")
		if err != nil {
			return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
		}
	}

	id, err := h.runtime.StartService(serviceName, &parent)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.runtime.StartService(service: '%s'): %v", serviceName, err))
	}

	return req.Ok(datatype.New().Set("id", id))
}

// onAddService registers a service target in the runtime configuration.
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

	err = h.runtime.AddService(target)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.runtime.AddService('%s'): %v", target.Name(), err))
	}

	return req.Ok(datatype.New())
}

// onSetService updates a service in the runtime configuration.
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

	err = h.runtime.SetService(service)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.runtime.SetService('%s'): %v", service.Name, err))
	}

	return req.Ok(datatype.New())
}

// onRemoveService removes a service from the runtime configuration.
// Requires 'service' string parameter with the service name.
func (h *Handler) onRemoveService(req message.RequestInterface) message.ReplyInterface {
	serviceName, err := req.RouteParameters().StringValue("service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('service'): %v", err))
	}

	err = h.runtime.RemoveService(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.runtime.RemoveService('%s'): %v", serviceName, err))
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

	err = h.runtime.StopService(serviceName)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.runtime.StopService: %v", err))
	}

	return req.Ok(datatype.New())
}

// Start starts the dependency handler with the available operations.
func (h *Handler) Start() error {
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

	return h.handler.Start()
}

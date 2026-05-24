// Package dep_handler creates a thread that manages the dependencies
package dep_handler

import (
	"fmt"

	clientConfig "github.com/sds-framework/client-lib/config"
	"github.com/sds-framework/datatype-lib/data_type/key_value"
	"github.com/sds-framework/datatype-lib/message"
	"github.com/sds-framework/dev-lib/runtime"
	"github.com/sds-framework/handler-lib/base"
	handlerConfig "github.com/sds-framework/handler-lib/config"
	"github.com/sds-framework/handler-lib/replier"
	"github.com/sds-framework/log-lib"
)

const (
	Category   = "dep_handler" // handler category
	DepRunning = "dep-running" // the command to check is dependency running
	RunDep     = "run-dep"     // the command to run the dependency
	CloseDep   = "close-dep"   // the command to stop the running dependency
)

type DepHandler struct {
	handler base.Interface
	runtime runtime.Interface
}

// ServiceConfig returns the socket configuration of the handler
func ServiceConfig() *handlerConfig.Handler {
	return handlerConfig.NewInternalHandler(handlerConfig.ReplierType, Category)
}

// New dep handler returned
func New(depRuntime runtime.Interface) (*DepHandler, error) {
	handler := replier.New()

	logger, err := log.New("dep_runtime", true)
	if err != nil {
		return nil, fmt.Errorf("log.New('dep-handler'): %w", err)
	}

	handler.SetConfig(ServiceConfig())
	err = handler.SetLogger(logger)
	if err != nil {
		return nil, fmt.Errorf("handler.SetLogger: %w", err)
	}

	return &DepHandler{
		runtime: depRuntime,
		handler: handler,
	}, nil
}

// onDepRunning checks whether the dependency is running or not.
// Requires:
//   - 'dep' of the clientConfig.Client.
//
// Returns 'running' boolean result
func (h *DepHandler) onDepRunning(req message.RequestInterface) message.ReplyInterface {
	kv, err := req.RouteParameters().NestedValue("dep")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetKeyValue('dep'): %v", err))
	}

	var c clientConfig.Client
	err = kv.Interface(&c)
	if err != nil {
		return req.Fail(fmt.Sprintf("kv.Interface: %v", err))
	}

	c.UrlFunc(clientConfig.Url)

	running, err := h.runtime.Running(&c)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.runtime.Running: %v", err))
	}

	params := key_value.New().Set("running", running)
	return req.Ok(params)
}

// onRunDep runs the dependency.
// Requires:
//   - 'url' string parameter,
//   - 'id' string parameter,
//   - 'parent' of the clientConfig.Client type.
//   - 'local_bin' string, optionally
//
// Returns nothing.
// todo make it publish the result through publisher, so user won't wait for the result.
func (h *DepHandler) onRunDep(req message.RequestInterface) message.ReplyInterface {
	kv, err := req.RouteParameters().NestedValue("parent")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetKeyValue('parent'): %v", err))
	}

	var parent clientConfig.Client
	err = kv.Interface(&parent)
	if err != nil {
		return req.Fail(fmt.Sprintf("kv.Interface: %v", err))
	}

	parent.UrlFunc(clientConfig.Url)

	url, err := req.RouteParameters().StringValue("url")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('url'): %v", err))
	}

	id, err := req.RouteParameters().StringValue("id")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetString('id'): %v", err))
	}

	optionalLocalBin, _ := req.RouteParameters().StringValue("local_bin")

	dep, err := runtime.NewDep(url, "", optionalLocalBin)
	if err != nil {
		return req.Fail(fmt.Sprintf("runtime.NewDep('%s', '', '%s'): %v", url, optionalLocalBin, err))
	}

	err = h.runtime.Run(dep, id, &parent)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.runtime.Run(url: '%s', id: '%s'): %v", url, id, err))
	}

	return req.Ok(key_value.New())
}

// onCloseDep stops the dependency.
// Requires 'dep' of the clientConfig.Client type.
// Returns nothing.
//
// Todo make it publish the result through publisher, so user won't wait for the result.
func (h *DepHandler) onCloseDep(req message.RequestInterface) message.ReplyInterface {
	kv, err := req.RouteParameters().NestedValue("dep")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.Parameters.GetKeyValue('dep'): %v", err))
	}

	var c clientConfig.Client
	err = kv.Interface(&c)
	if err != nil {
		return req.Fail(fmt.Sprintf("kv.Interface: %v", err))
	}

	c.UrlFunc(clientConfig.Url)

	err = h.runtime.Close(&c)
	if err != nil {
		return req.Fail(fmt.Sprintf("h.runtime.Close: %v", err))
	}

	return req.Ok(key_value.New())
}

// Start the dependency handler with the available operations.
func (h *DepHandler) Start() error {
	if err := h.handler.Route(DepRunning, h.onDepRunning); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", DepRunning, err)
	}
	if err := h.handler.Route(RunDep, h.onRunDep); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", RunDep, err)
	}
	if err := h.handler.Route(CloseDep, h.onCloseDep); err != nil {
		return fmt.Errorf("h.handler.Route('%s'): %v", CloseDep, err)
	}

	return h.handler.Start()
}

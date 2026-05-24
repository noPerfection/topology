// Package context sets up the developer context.
package context

import (
	"fmt"

	config "github.com/sds-framework/config-lib"
	"github.com/sds-framework/dev-lib/dep_client"
	"github.com/sds-framework/dev-lib/dep_handler"
	"github.com/sds-framework/dev-lib/dep_manager"
	"github.com/sds-framework/dev-lib/proxy_client"
	"github.com/sds-framework/dev-lib/proxy_handler"
	"github.com/sds-framework/handler-lib/manager_client"
	"github.com/sds-framework/log-lib"
)

// A Context handles the config of the contexts
type Context struct {
	Config              config.SdsService
	depHandler          *dep_handler.DepHandler
	depHandlerManager   manager_client.Interface
	proxyClient         proxy_client.Interface
	proxyHandler        *proxy_handler.ProxyHandler
	proxyHandlerManager manager_client.Interface
	serviceId           string
	serviceUrl          string
	depClient           *dep_client.Client
}

// NewDev creates Developer context.
// Loads it with the Dev Configuration and Dev DepManager Manager.
func NewDev(configPath string) (*Context, error) {
	ctx := &Context{}

	appConfig, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("config.Load('%s'): %w", configPath, err)
	}
	ctx.Config = appConfig

	return ctx, nil
}

func (ctx *Context) IsRunning() bool {
	return ctx.depHandlerManager != nil && ctx.proxyHandlerManager != nil
}

func (ctx *Context) IsDepManagerRunning() bool {
	return ctx.depHandlerManager != nil
}

func (ctx *Context) IsProxyHandlerRunning() bool {
	return ctx.proxyHandlerManager != nil
}

func (ctx *Context) SetDepClient(dc dep_client.Interface) error {
	devDc, ok := dc.(*dep_client.Client)
	if !ok {
		return fmt.Errorf("only dev context dep client supported")
	}
	ctx.depClient = devDc

	return nil
}

func (ctx *Context) DepClient() dep_client.Interface {
	return ctx.depClient
}

// SetProxyClient sets the client that works with proxies
func (ctx *Context) SetProxyClient(proxyClient proxy_client.Interface) error {
	ctx.proxyClient = proxyClient

	return nil
}

// ProxyClient returns the client that works with proxies
func (ctx *Context) ProxyClient() proxy_client.Interface {
	return ctx.proxyClient
}

// Type returns the context type. Useful to identify contexts in the generic functions.
func (ctx *Context) Type() ContextType {
	return DevContext
}

// Close the dep and proxy handlers. The dep manager client is not closed.
func (ctx *Context) Close() error {
	if ctx.proxyHandlerManager != nil {
		if ctx.proxyHandlerManager != nil {
			if err := ctx.proxyHandlerManager.Close(); err != nil {
				return fmt.Errorf("ctx.proxyHandlerManager.Close: %w", err)
			}
		}
		ctx.proxyHandlerManager = nil
	}

	if ctx.depHandlerManager != nil {
		if err := ctx.depHandlerManager.Close(); err != nil {
			return fmt.Errorf("ctx.depHandlerManager.Close: %w", err)
		}
		ctx.depHandlerManager = nil
	}

	return nil
}

// SetService sets the service id and url for which this context belongs too.
func (ctx *Context) SetService(id string, url string) {
	ctx.serviceId = id
	ctx.serviceUrl = url
}

// StartDepManager starts the dependency manager
func (ctx *Context) StartDepManager() error {
	if ctx.depHandlerManager != nil {
		return fmt.Errorf("dep manager already started")
	}
	srcPath, binPath, err := DevDefaultPaths()
	if err != nil {
		return fmt.Errorf("DevDefaultPaths: %w", err)
	}

	//
	// Start the dependency manager
	//
	depManager := dep_manager.New()
	if err := depManager.SetPaths(binPath, srcPath); err != nil {
		return fmt.Errorf("depManager.SetPaths('%s', '%s'): %w", binPath, srcPath, err)
	}
	ctx.depHandler, err = dep_handler.New(depManager)
	if err != nil {
		return fmt.Errorf("dep_handler.New: %w", err)
	}

	err = ctx.depHandler.Start()
	if err != nil {
		return fmt.Errorf("depHandler: %w", err)
	}

	ctx.depHandlerManager, err = manager_client.New(dep_handler.ServiceConfig())
	if err != nil {
		return fmt.Errorf("manager_client.New('dep_handler'): %w", err)
	}

	depClient, err := dep_client.New()
	if err != nil {
		return fmt.Errorf("dep_client.New: %w", err)
	}

	err = ctx.SetDepClient(depClient)
	if err != nil {
		return fmt.Errorf("ctx.SetDepClient: %w", err)
	}

	return nil
}

// StartProxyHandler starts the proxy handler
func (ctx *Context) StartProxyHandler() error {
	if len(ctx.serviceId) == 0 || len(ctx.serviceUrl) == 0 {
		return fmt.Errorf("service parameters are not set. call Context.SetService first")
	}
	if ctx.proxyHandlerManager != nil {
		return fmt.Errorf("proxy handler already started")
	}
	proxyLogger, err := log.New("proxy-handler", true)
	if err != nil {
		return fmt.Errorf("log.New('proxy-handler'): %w", err)
	}

	depClient, err := dep_client.New()
	if err != nil {
		return fmt.Errorf("dep_client.New: %w", err)
	}

	proxyHandler := proxy_handler.New(&ctx.Config, depClient)
	proxyHandlerConfig := proxy_handler.HandlerConfig(ctx.serviceId)
	proxyHandler.SetConfig(proxyHandlerConfig)
	err = proxyHandler.SetLogger(proxyLogger)
	if err != nil {
		return fmt.Errorf("proxyHandler.SetLogger: %w", err)
	}
	proxyHandler.SetServiceId(ctx.serviceId)
	err = proxyHandler.Start()
	if err != nil {
		return fmt.Errorf("proxyHandler.Start: %w", err)
	}
	ctx.proxyHandler = proxyHandler

	ctx.proxyHandlerManager, err = manager_client.New(proxyHandlerConfig)
	if err != nil {
		return fmt.Errorf("manager_client.New('proxyHandlerConfig'): %w", err)
	}
	proxyClient, err := proxy_client.New(ctx.serviceId)
	if err != nil {
		return fmt.Errorf("proxy_client.New('%s'): %w", ctx.serviceId, err)
	}
	err = ctx.SetProxyClient(proxyClient)
	if err != nil {
		return fmt.Errorf("ctx.SetProxyClient: %w", err)
	}

	return nil
}

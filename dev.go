// Package context sets up the developer context.
package context

import (
	"fmt"

	config "github.com/sds-framework/config-lib"
	"github.com/sds-framework/dev-lib/dep_client"
	"github.com/sds-framework/dev-lib/dep_handler"
	"github.com/sds-framework/dev-lib/runtime"
	"github.com/sds-framework/handler-lib/manager_client"
)

// A Context handles the config of the contexts
type Context struct {
	Config            config.SdsService
	depHandler        *dep_handler.DepHandler
	depHandlerManager manager_client.Interface
	depClient         *dep_client.Client
}

// NewDev creates Developer context.
// Loads it with the Dev Configuration.
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
	return ctx.depHandlerManager != nil
}

func (ctx *Context) IsDepManagerRunning() bool {
	return ctx.depHandlerManager != nil
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

// Type returns the context type. Useful to identify contexts in the generic functions.
func (ctx *Context) Type() ContextType {
	return DevContext
}

// Close the dep handler. The dep manager client is not closed.
func (ctx *Context) Close() error {
	if ctx.depHandlerManager != nil {
		if err := ctx.depHandlerManager.Close(); err != nil {
			return fmt.Errorf("ctx.depHandlerManager.Close: %w", err)
		}
		ctx.depHandlerManager = nil
	}

	return nil
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
	// Start the dependency runtime
	//
	depRuntime := runtime.New()
	if err := depRuntime.SetPaths(binPath, srcPath); err != nil {
		return fmt.Errorf("depRuntime.SetPaths('%s', '%s'): %w", binPath, srcPath, err)
	}
	ctx.depHandler, err = dep_handler.New(depRuntime)
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

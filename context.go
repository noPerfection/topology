// Package context sets up the developer context.
package context

import (
	"fmt"

	config "github.com/noPerfection/context/config"
	"github.com/noPerfection/context/runtime"
)

// A Context handles the config of the contexts
type Context struct {
	Config         config.NoPerfection
	runtimeHandler *runtime.Handler
	runtimeClient  *runtime.Client
}

// New creates a developer context and loads it with the dev configuration.
func New(configPath string, runtimeSocket config.Socket) (*Context, error) {
	ctx := &Context{}

	appConfig, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("config.Load('%s'): %w", configPath, err)
	}
	ctx.Config = appConfig

	appConfigChanged, err := ensureIndependentRuntimeService(&appConfig, runtimeSocket)
	if err != nil {
		return nil, fmt.Errorf("ensureIndependentRuntimeService: %w", err)
	}
	if appConfigChanged {
		if err := appConfig.Save(); err != nil {
			return nil, fmt.Errorf("appConfig.Save: %w", err)
		}
	}
	ctx.Config = appConfig

	ctx.runtimeHandler, err = runtime.NewHandler(&ctx.Config, runtimeSocket)
	if err != nil {
		return nil, fmt.Errorf("runtime.NewHandler: %w", err)
	}

	runtimeAccess, err := runtime.NewClient(runtimeSocket)
	if err != nil {
		return nil, fmt.Errorf("runtime.NewClient: %w", err)
	}

	ctx.runtimeClient = runtimeAccess

	return ctx, nil
}

func ensureIndependentRuntimeService(appConfig *config.NoPerfection, runtimeSocket config.Socket) (bool, error) {
	independentCount := appConfig.CountByType(config.IndependentType)
	if independentCount > 1 {
		return false, fmt.Errorf("only one independent service can be configured")
	}

	runtimeHandler := config.Handler{
		Type:     config.HandlerType(runtime.RuntimeSocketType),
		Category: runtime.RuntimeHandlerCategory,
		Socket:   runtimeSocket,
	}

	if independentCount == 0 {
		err := appConfig.SetService(config.Service{
			Type:     config.IndependentType,
			Name:     runtime.RuntimeHandlerCategory,
			Handlers: []config.Handler{runtimeHandler},
		})
		if err != nil {
			return false, fmt.Errorf("appConfig.SetService: %w", err)
		}

		return true, nil
	}

	independentService, err := appConfig.GetByType(config.IndependentType)
	if err != nil {
		return false, fmt.Errorf("appConfig.GetByType('%s'): %w", config.IndependentType, err)
	}

	handler, err := independentService.HandlerByCategory(runtime.RuntimeHandlerCategory)
	if err == nil {
		if handler.Socket.Id == runtimeSocket.Id && handler.Socket.Port == runtimeSocket.Port {
			return false, nil
		}

		handler.Socket = runtimeSocket
		independentService.SetHandler(handler)
		return true, nil
	}

	independentService.Handlers = append(independentService.Handlers, runtimeHandler)
	return true, nil
}

func (ctx *Context) Runtime() runtime.ClientInterface {
	return ctx.runtimeClient
}

// StartRuntimeHandler starts the runtime handler.
func (ctx *Context) StartRuntimeHandler() error {
	if ctx.runtimeHandler == nil {
		return fmt.Errorf("runtime handler not initialized")
	}

	err := ctx.runtimeHandler.Start()
	if err != nil {
		return fmt.Errorf("runtimeHandler: %w", err)
	}

	return nil
}

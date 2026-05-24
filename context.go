package context

import (
	"fmt"
	config "github.com/sds-framework/config-lib"
	"github.com/sds-framework/dev-lib/dep_client"
	"github.com/sds-framework/dev-lib/proxy_client"
	"github.com/sds-framework/os-lib/arg"
)

type Interface interface {
	SetConfig(p *config.SdsService)
	SetProxyClient(p proxy_client.Interface) error
	ProxyClient() proxy_client.Interface
	Type() ContextType
	StartDepManager() error
	StartProxyHandler() error
	Close() error // Close the dep and proxy handlers. The dep manager client is not closed.
	IsRunning() bool
	IsDepManagerRunning() bool
	IsProxyHandlerRunning() bool
	SetService(string, string) // SetService sets the service parameters
	SetDepClient(p dep_client.Interface) error
	DepClient() dep_client.Interface
}

// A New orchestra. Optionally pass the config path and/or type of the context.
// Or the context type could be retrieved from the config.ContextFlag.
func New(args ...string) (Interface, error) {
	ctxType := DevContext // default is used a dev context
	configPath := ""

	for _, value := range args {
		if value == DevContext || value == UnknownContext {
			ctxType = value
			continue
		}

		configPath = value
	}

	if len(args) == 0 && arg.FlagExist(ContextFlag) {
		ctxType = arg.FlagValue(ContextFlag)
	}
	if len(configPath) == 0 && arg.FlagExist(ConfigFlag) {
		configPath = arg.FlagValue(ConfigFlag)
	}

	if ctxType == DevContext {
		return NewDev(configPath)
	}

	return nil, fmt.Errorf("only %s supported, not %s", DevContext, ctxType)
}

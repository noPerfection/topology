package topology

import (
	"fmt"

	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology/config"
)

func newHandler(appConfig *config.NoPerfection, runtimeEndpoint message.Endpoint) (*Handler, error) {
	if appConfig == nil {
		appConfig = &config.NoPerfection{}
	}

	handler := replier.New()

	logger, err := log.New(RuntimeHandlerCategory, true)
	if err != nil {
		return nil, fmt.Errorf("log.New('%s'): %w", RuntimeHandlerCategory, err)
	}

	handler.SetConfig(HandlerConfig(runtimeEndpoint))
	if err := handler.SetLogger(logger); err != nil {
		return nil, fmt.Errorf("handler.SetLogger: %w", err)
	}

	return &Handler{
		runtime: New(appConfig),
		handler: handler,
	}, nil
}

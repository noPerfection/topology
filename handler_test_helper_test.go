package topology

import (
	"fmt"

	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/handler/replier"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology/config"
)

func newHandler(appConfig *config.NoPerfection, topologyEndpoint message.Endpoint) (*Handler, error) {
	if appConfig == nil {
		appConfig = &config.NoPerfection{}
	}

	handler := replier.New()

	logger, err := log.New(TopologyHandlerCategory, true)
	if err != nil {
		return nil, fmt.Errorf("log.New('%s'): %w", TopologyHandlerCategory, err)
	}

	handler.SetConfig(HandlerConfig(topologyEndpoint))
	if err := handler.SetLogger(logger); err != nil {
		return nil, fmt.Errorf("handler.SetLogger: %w", err)
	}

	return &Handler{
		topology: New(appConfig),
		handler:  handler,
	}, nil
}

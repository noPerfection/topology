// Package proxy_handler is a thread that manages the service proxies
package proxy_handler

import (
	"fmt"
	"slices"

	config "github.com/sds-framework/config-lib"
	"github.com/sds-framework/datatype-lib/data_type/key_value"
	"github.com/sds-framework/datatype-lib/message"
	"github.com/sds-framework/dev-lib/dep_client"
	service "github.com/sds-framework/dev-lib/proxy_config"
	"github.com/sds-framework/handler-lib/base"
	handlerConfig "github.com/sds-framework/handler-lib/config"
)

const (
	Category = "proxy_handler" // handler category

	//
	// Commands of the proxy handler
	//

	SetProxyChain       = "set-proxy-chain"         // SetProxyChain is a route command that sets a new proxy chain
	SetUnits            = "set-units"               // route command that sets the proxy units
	ProxyChainsByLastId = "proxy-chains-by-last-id" // returns list of proxy chains by the id of the last proxy
	Units               = "units"                   // route command that returns a list of destination units for a rule
	LastProxies         = "last-proxies"            // that returns a list of last proxies in the proxy chains.
	StartLastProxies    = "start-last-proxies"      // route command that starts all proxies
	ProxyChainByRule    = "proxy-chain-by-rule"     // route command that returns a proxy chains by the rule
	ProxyChains         = "proxy-chains"            // returns list of proxy chains
)

type ProxyHandler struct {
	*base.Handler
	proxyChains []*service.ProxyChain
	proxyUnits  map[*service.Rule][]*service.Unit
	depClient   dep_client.Interface
	Config      *config.SdsService
	serviceId   string
}

// Id of the proxy handler based on the service id
func Id(id string) string {
	return fmt.Sprintf("%s_proxy_handler", id)
}

// HandlerConfig creates a configuration of the thread
func HandlerConfig(serviceId string) *handlerConfig.Handler {
	inprocConfig := handlerConfig.NewInternalHandler(handlerConfig.SyncReplierType, Category)
	inprocConfig.Id = Id(serviceId)

	return inprocConfig
}

// New returns a proxy handler
func New(appConfig *config.SdsService, depClient dep_client.Interface) *ProxyHandler {
	newHandler := base.New()
	return &ProxyHandler{
		Handler:     newHandler,
		proxyChains: make([]*service.ProxyChain, 0),
		proxyUnits:  make(map[*service.Rule][]*service.Unit, 0),
		Config:      appConfig,
		depClient:   depClient,
	}
}

// SetServiceId notifies the proxy handler with the service where it's belonged too.
func (proxyHandler *ProxyHandler) SetServiceId(id string) {
	proxyHandler.serviceId = id
}

func (proxyHandler *ProxyHandler) setUnits(rule *service.Rule, units []*service.Unit) {
	for firstRule := range proxyHandler.proxyUnits {
		if service.IsEqualRule(firstRule, rule) {
			proxyHandler.proxyUnits[firstRule] = units
			return
		}
	}
	proxyHandler.proxyUnits[rule] = units
}

// The Route is over-written to be disabled.
func (proxyHandler *ProxyHandler) Route(string, any, ...string) error {
	return fmt.Errorf("not implemented")
}

// onSetProxyChain is a handler function to set the new proxy chain
func (proxyHandler *ProxyHandler) onSetProxyChain(req message.RequestInterface) message.ReplyInterface {
	raw, err := req.RouteParameters().NestedValue("proxy_chain")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('proxy_chain'): %v", err))
	}

	var proxyChain service.ProxyChain
	err = raw.Interface(&proxyChain)
	if err != nil {
		return req.Fail(fmt.Sprintf("key_value.KeyValue('proxy_chain').Interface(): %v", err))
	}

	// Feel the missing urls
	if proxyChain.Sources == nil {
		proxyChain.Sources = []string{}
	}

	if !proxyChain.IsValid() {
		return req.Fail("proxy chain is not valid")
	}

	params := key_value.New()

	i := slices.IndexFunc(proxyHandler.proxyChains, func(proxyChain *service.ProxyChain) bool {
		return service.IsEqualRule(proxyChain.Destination, proxyChain.Destination)
	})
	if i > -1 {
		proxyHandler.proxyChains[i] = &proxyChain
	} else {
		proxyHandler.proxyChains = append(proxyHandler.proxyChains, &proxyChain)
	}
	params.Set("overwrite", i > -1)

	return req.Ok(key_value.New())
}

// onProxyChainByRule is a handler function to get the proxy chain by the destination.
//
// This method is intended to be called by the independent service, to return the list of proxy chains in the service.
//
// Returns empty data if no proxy chain is found.
func (proxyHandler *ProxyHandler) onProxyChainByRule(req message.RequestInterface) message.ReplyInterface {
	raw, err := req.RouteParameters().NestedValue("rule")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('rule'): %v", err))
	}
	var rule service.Rule
	err = raw.Interface(&rule)
	if err != nil {
		return req.Fail(fmt.Sprintf("raw.Interface('service.Rule'): %v", err))
	}

	proxyChain := service.ProxyChainByRule(proxyHandler.proxyChains, &rule)
	if proxyChain == nil {
		proxyChain = &service.ProxyChain{
			Sources:     []string{},
			Proxies:     []*service.Proxy{},
			Destination: service.NewServiceDestination(), // service.NewServiceDestination returns empty rule
		}
	}
	params := key_value.New().Set("proxy_chain", proxyChain)

	return req.Ok(params)
}

// onSetUnits is a handler function that sets the list of all proxy units for each rule.
func (proxyHandler *ProxyHandler) onSetUnits(req message.RequestInterface) message.ReplyInterface {
	raw, err := req.RouteParameters().NestedValue("rule")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('proxy_chain'): %v", err))
	}

	var rule service.Rule
	err = raw.Interface(&rule)
	if err != nil {
		return req.Fail(fmt.Sprintf("key_value.KeyValue('proxy_chain').Interface(): %v", err))
	}

	if !rule.IsValid() {
		return req.Fail("the 'rule' parameter is not valid")
	}

	rawUnits, err := req.RouteParameters().NestedListValue("units")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedListValue('units'): %v", err))
	}

	units := make([]*service.Unit, len(rawUnits))
	for i, rawUnit := range rawUnits {
		var unit service.Unit
		err = rawUnit.Interface(&unit)
		if err != nil {
			return req.Fail(fmt.Sprintf("rawUnits[%d].Interface: %v", i, err))
		}

		units[i] = &unit
	}

	proxyHandler.setUnits(&rule, units)

	return req.Ok(key_value.New())
}

// onProxyChainsByLastId returns list of proxy chains where the proxy id is in the proxies list
func (proxyHandler *ProxyHandler) onProxyChainsByLastId(req message.RequestInterface) message.ReplyInterface {
	id, err := req.RouteParameters().StringValue("id")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('id'): %v", err))
	}

	proxyChains := make([]*service.ProxyChain, 0, len(proxyHandler.proxyChains))
	for _, proxyChain := range proxyHandler.proxyChains {
		lastProxy := len(proxyChain.Proxies) - 1
		if lastProxy == -1 {
			continue
		}

		if proxyChain.Proxies[lastProxy].Id == id {
			proxyChains = append(proxyChains, proxyChain)
		}
	}

	params := key_value.New().Set("proxy_chains", proxyChains)

	return req.Ok(params)
}

// onUnits returns the list of units by a rule
func (proxyHandler *ProxyHandler) onUnits(req message.RequestInterface) message.ReplyInterface {
	raw, err := req.RouteParameters().NestedValue("rule")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('proxy_chain'): %v", err))
	}

	var rule service.Rule
	err = raw.Interface(&rule)
	if err != nil {
		return req.Fail(fmt.Sprintf("key_value.KeyValue('proxy_chain').Interface(): %v", err))
	}

	if !rule.IsValid() {
		return req.Fail("the 'rule' parameter is not valid")
	}

	units := make([]*service.Unit, 0)

	for firstRule := range proxyHandler.proxyUnits {
		if !service.IsEqualRule(firstRule, &rule) {
			continue
		}
		for i := range proxyHandler.proxyUnits[firstRule] {
			unit := proxyHandler.proxyUnits[firstRule][i]
			units = append(units, unit)
		}
		break
	}

	params := key_value.New().Set("units", units)

	return req.Ok(params)
}

// onLastProxies returns the list of the proxies.
// The duplicate proxies are compacted
func (proxyHandler *ProxyHandler) onLastProxies(req message.RequestInterface) message.ReplyInterface {
	proxies := service.LastProxies(proxyHandler.proxyChains)

	params := key_value.New().Set("proxies", proxies)

	return req.Ok(params)
}

// onStartLastProxies starts the proxies
func (proxyHandler *ProxyHandler) onStartLastProxies(req message.RequestInterface) message.ReplyInterface {
	if len(proxyHandler.serviceId) == 0 {
		return req.Fail("serviceId not set. call ProxyHandler.SetServiceId first")
	}
	if proxyHandler.Config == nil {
		return req.Fail("config is not set")
	}
	if proxyHandler.depClient == nil {
		return req.Fail("dependency manager is not set")
	}
	depManager := proxyHandler.depClient

	proxies := service.LastProxies(proxyHandler.proxyChains)

	if len(proxies) == 0 {
		return req.Ok(key_value.New())
	}

	if _, err := proxyHandler.Config.GetService(proxyHandler.serviceId); err != nil {
		return req.Fail(fmt.Sprintf("config.GetService('%s'): %v", proxyHandler.serviceId, err))
	}

	for i := range proxies {
		proxy := proxies[i]

		if err := depManager.Run(proxy.Url, proxy.Id, nil, proxy.LocalBin); err != nil {
			return req.Fail(fmt.Sprintf("depManager.Run('%s', '%s'): %v", proxy.Url, proxy.Id, err))
		}
	}

	return req.Ok(key_value.New())
}

// onProxyChains returns all proxy chains
func (proxyHandler *ProxyHandler) onProxyChains(req message.RequestInterface) message.ReplyInterface {
	proxyChains := make([]*service.ProxyChain, 0, len(proxyHandler.proxyChains))
	proxyChains = append(proxyChains, proxyHandler.proxyChains...)

	params := key_value.New().Set("proxy_chains", proxyChains)

	return req.Ok(params)
}

func (proxyHandler *ProxyHandler) onClose(request message.RequestInterface) message.ReplyInterface {
	if err := proxyHandler.closeProxies(); err != nil {
		return request.Fail(fmt.Sprintf("proxyHandler.closeProxies: %v", err))
	}

	proxyHandler.proxyChains = make([]*service.ProxyChain, 0)
	proxyHandler.proxyUnits = make(map[*service.Rule][]*service.Unit, 0)

	return proxyHandler.Handler.Manager.SetClose(request)
}

func (proxyHandler *ProxyHandler) setRoutes() error {
	if err := proxyHandler.Handler.Route(SetProxyChain, proxyHandler.onSetProxyChain); err != nil {
		return fmt.Errorf("proxyHandler.Handler.Route('%s'): %w", SetProxyChain, err)
	}
	if err := proxyHandler.Handler.Route(ProxyChainByRule, proxyHandler.onProxyChainByRule); err != nil {
		return fmt.Errorf("proxyHandler.Handler.Route('%s'): %w", ProxyChainByRule, err)
	}
	if err := proxyHandler.Handler.Route(SetUnits, proxyHandler.onSetUnits); err != nil {
		return fmt.Errorf("proxyHandler.Handler.Route('%s'): %w", SetUnits, err)
	}
	if err := proxyHandler.Handler.Route(ProxyChainsByLastId, proxyHandler.onProxyChainsByLastId); err != nil {
		return fmt.Errorf("proxyHandler.Handler.Route('%s'): %w", ProxyChainsByLastId, err)
	}
	if err := proxyHandler.Handler.Route(Units, proxyHandler.onUnits); err != nil {
		return fmt.Errorf("proxyHandler.Handler.Route('%s'): %w", Units, err)
	}
	if err := proxyHandler.Handler.Route(LastProxies, proxyHandler.onLastProxies); err != nil {
		return fmt.Errorf("proxyHandler.Handler.Route('%s'): %w", LastProxies, err)
	}
	if err := proxyHandler.Handler.Route(StartLastProxies, proxyHandler.onStartLastProxies); err != nil {
		return fmt.Errorf("proxyHandler.Handler.Route('%s'): %w", StartLastProxies, err)
	}
	if err := proxyHandler.Handler.Route(ProxyChains, proxyHandler.onProxyChains); err != nil {
		return fmt.Errorf("proxyHandler.Handler.Route('%s'): %w", ProxyChains, err)
	}

	return nil
}

// closeProxies will close all proxies that have generated configuration
func (proxyHandler *ProxyHandler) closeProxies() error {
	if len(proxyHandler.serviceId) == 0 {
		return fmt.Errorf("serviceId not set. call ProxyHandler.SetServiceId first")
	}
	if proxyHandler.Config == nil {
		return nil
	}
	if proxyHandler.depClient == nil {
		return nil
	}

	return nil
}

// Start starts the proxy handler as a new thread
func (proxyHandler *ProxyHandler) Start() error {
	if len(proxyHandler.Handler.Routes) > 0 {
		return fmt.Errorf("writing routes is not allowed")
	}
	if len(proxyHandler.serviceId) == 0 {
		return fmt.Errorf("missing serviceId. Call ProxyHandler.SetServiceId first")
	}

	if err := proxyHandler.setRoutes(); err != nil {
		return fmt.Errorf("proxyHandler.setRoutes: %w", err)
	}

	if proxyHandler.Handler.Manager != nil {
		if err := proxyHandler.Handler.Manager.Route(handlerConfig.HandlerClose, proxyHandler.onClose); err != nil {
			return fmt.Errorf("manager.Route('%s'): %w", handlerConfig.HandlerClose, err)
		}
	}

	if err := proxyHandler.Handler.Start(); err != nil {
		return fmt.Errorf("proxyHandler.Handler.Start: %w", err)
	}

	return nil
}

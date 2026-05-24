package proxy_config

import "slices"

type Local struct{}

type Proxy struct {
	Id       string `json:"id"`
	Url      string `json:"url"`
	Category string `json:"category"`
	Local    *Local `json:"local,omitempty"`
	LocalSrc string `json:"local_src,omitempty"`
	LocalBin string `json:"local_bin,omitempty"`
}

type Rule struct {
	Urls             []string `json:"urls"`
	Categories       []string `json:"categories"`
	Commands         []string `json:"commands"`
	ExcludedCommands []string `json:"excluded_commands"`
}

type Unit struct {
	ServiceId string `json:"service_id"`
	HandlerId string `json:"handler_id"`
	Command   string `json:"command"`
}

type ProxyChain struct {
	Sources     []string `json:"sources"`
	Proxies     []*Proxy `json:"proxies"`
	Destination *Rule    `json:"destination"`
}

func NewServiceDestination(urls ...string) *Rule {
	return &Rule{Urls: urls, Categories: []string{}, Commands: []string{}, ExcludedCommands: []string{}}
}

func NewHandlerDestination(categories ...string) *Rule {
	return &Rule{Urls: []string{}, Categories: categories, Commands: []string{}, ExcludedCommands: []string{}}
}

func (rule *Rule) IsEmpty() bool {
	if rule == nil {
		return true
	}

	return len(rule.Urls) == 0 &&
		len(rule.Categories) == 0 &&
		len(rule.Commands) == 0 &&
		len(rule.ExcludedCommands) == 0
}

func (rule *Rule) IsValid() bool {
	return !rule.IsEmpty()
}

func IsEqualRule(a *Rule, b *Rule) bool {
	if a == nil || b == nil {
		return a == b
	}

	return slices.Equal(a.Urls, b.Urls) &&
		slices.Equal(a.Categories, b.Categories) &&
		slices.Equal(a.Commands, b.Commands) &&
		slices.Equal(a.ExcludedCommands, b.ExcludedCommands)
}

func (chain *ProxyChain) IsValid() bool {
	if chain == nil {
		return false
	}

	return chain.Destination.IsValid() && len(chain.Proxies) > 0
}

func ProxyChainByRule(chains []*ProxyChain, rule *Rule) *ProxyChain {
	i := slices.IndexFunc(chains, func(chain *ProxyChain) bool {
		return chain != nil && IsEqualRule(chain.Destination, rule)
	})
	if i == -1 {
		return nil
	}

	return chains[i]
}

func LastProxies(chains []*ProxyChain) []*Proxy {
	proxies := make([]*Proxy, 0, len(chains))
	seen := make(map[string]struct{}, len(chains))

	for _, chain := range chains {
		if chain == nil || len(chain.Proxies) == 0 {
			continue
		}

		proxy := chain.Proxies[len(chain.Proxies)-1]
		if proxy == nil {
			continue
		}
		if _, ok := seen[proxy.Id]; ok {
			continue
		}

		seen[proxy.Id] = struct{}{}
		proxies = append(proxies, proxy)
	}

	return proxies
}

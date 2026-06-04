package config

import "fmt"

// Normalize registers inline services from handler-deps and command-deps into
// Services and validates that every dependency target name resolves.
func (a *NoPerfection) Normalize() error {
	if a == nil {
		return fmt.Errorf("app struct is nil")
	}

	visiting := make(map[string]bool)
	for i := range a.Services {
		if err := a.normalizeRecord(&a.Services[i], visiting); err != nil {
			return fmt.Errorf("service %q: %w", a.Services[i].Name, err)
		}
	}

	return a.validateDepRefs()
}

func (a *NoPerfection) normalizeRecord(record *ServiceRecord, visiting map[string]bool) error {
	if record == nil {
		return fmt.Errorf("service record is nil")
	}
	if record.Proxy != nil {
		if err := record.Proxy.ValidateTypes(); err != nil {
			return err
		}
		record.Service = serviceFromProxy(*record.Proxy)
		if err := a.normalizeService(&record.Service, visiting); err != nil {
			return err
		}
		for hi := range record.Proxy.Handlers {
			for oi := range record.Proxy.Handlers[hi].Outbounds {
				target := &record.Proxy.Handlers[hi].Outbounds[oi]
				if err := a.normalizeDepTarget(target, visiting); err != nil {
					return fmt.Errorf("outbound %q: %w", record.Proxy.Handlers[hi].Category, err)
				}
			}
		}
		return nil
	}
	return a.normalizeService(&record.Service, visiting)
}

func (a *NoPerfection) normalizeService(service *Service, visiting map[string]bool) error {
	if service == nil {
		return fmt.Errorf("service is nil")
	}
	if service.Name == "" {
		return fmt.Errorf("service name is empty")
	}
	if visiting[service.Name] {
		return fmt.Errorf("cycle detected at service %q", service.Name)
	}
	visiting[service.Name] = true
	defer delete(visiting, service.Name)

	if err := service.ValidateTypes(); err != nil {
		return err
	}

	for di := range service.HandlerDeps {
		dep := &service.HandlerDeps[di]
		if err := a.normalizeDepService(dep, visiting); err != nil {
			return fmt.Errorf("handler-deps category %q: %w", dep.Name, err)
		}
	}

	for hi := range service.Handlers {
		for di := range service.Handlers[hi].CommandDeps {
			dep := &service.Handlers[hi].CommandDeps[di]
			if err := a.normalizeDepService(dep, visiting); err != nil {
				return fmt.Errorf("command %q: %w", dep.Name, err)
			}
		}
	}
	return nil
}

func (a *NoPerfection) normalizeDepService(dep *DepService, visiting map[string]bool) error {
	if err := ValidateDepService(*dep); err != nil {
		return err
	}

	for i := range dep.Proxies {
		if err := a.normalizeDepTarget(&dep.Proxies[i], visiting); err != nil {
			return fmt.Errorf("proxies[%d]: %w", i, err)
		}
	}
	for i := range dep.Extensions {
		if err := a.normalizeDepTarget(&dep.Extensions[i], visiting); err != nil {
			return fmt.Errorf("extensions[%d]: %w", i, err)
		}
	}
	return nil
}

func (a *NoPerfection) normalizeDepTarget(target *DepTarget, visiting map[string]bool) error {
	if err := ValidateDepTarget(*target); err != nil {
		return err
	}

	if target.Ref != "" {
		return nil
	}

	record := target.ServiceRecord
	if err := a.normalizeRecord(&record, visiting); err != nil {
		return err
	}
	return a.SetService(record)
}

func (a *NoPerfection) validateDepRefs() error {
	for _, service := range a.Services {
		for _, dep := range service.HandlerDeps {
			if err := a.validateDepServiceRefs(dep); err != nil {
				return fmt.Errorf("service %q handler-deps category %q: %w", service.Name, dep.Name, err)
			}
		}
		for _, handler := range service.Handlers {
			for _, dep := range handler.CommandDeps {
				if err := a.validateDepServiceRefs(dep); err != nil {
					return fmt.Errorf("service %q command %q: %w", service.Name, dep.Name, err)
				}
			}
		}
	}
	return nil
}

func (a *NoPerfection) validateDepServiceRefs(dep DepService) error {
	for _, target := range dep.Proxies {
		if err := a.validateDepRef(target); err != nil {
			return fmt.Errorf("proxy: %w", err)
		}
	}
	for _, target := range dep.Extensions {
		if err := a.validateDepRef(target); err != nil {
			return fmt.Errorf("extension: %w", err)
		}
	}
	return nil
}

func (a *NoPerfection) validateDepRef(target DepTarget) error {
	if target.Ref != "" {
		serviceName, handlerCategory := target.RefPath()
		if serviceName == "" {
			return fmt.Errorf("dep target service name is empty")
		}
		record, err := a.GetService(serviceName)
		if err != nil {
			return fmt.Errorf("service %q not found: %w", serviceName, err)
		}
		if handlerCategory == "" {
			return nil
		}
		if _, err := record.HandlerByCategory(handlerCategory); err != nil {
			return fmt.Errorf("service %q handler category %q: %w", serviceName, handlerCategory, err)
		}
		return nil
	}

	name := target.Name()
	if name == "" {
		return fmt.Errorf("dep target name is empty")
	}
	if _, err := a.GetService(name); err != nil {
		return fmt.Errorf("service %q not found: %w", name, err)
	}
	return nil
}

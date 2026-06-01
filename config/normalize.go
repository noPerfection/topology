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
		if err := a.normalizeService(&a.Services[i], visiting); err != nil {
			return fmt.Errorf("service %q: %w", a.Services[i].Name, err)
		}
	}

	return a.validateDepRefs()
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

	if err := a.SetService(*service); err != nil {
		return err
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

	if err := a.normalizeService(target.Inline, visiting); err != nil {
		return err
	}
	return a.SetService(*target.Inline)
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
	name := target.Name()
	if name == "" {
		return fmt.Errorf("dep target name is empty")
	}
	if _, err := a.GetService(name); err != nil {
		return fmt.Errorf("service %q not found: %w", name, err)
	}
	return nil
}

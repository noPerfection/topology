package config

import "fmt"

// Normalize registers inline services from command-deps into Services and
// validates that every dependency target name resolves to a known service.
func (a *SdsService) Normalize() error {
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

func (a *SdsService) normalizeService(service *Service, visiting map[string]bool) error {
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

	for hi := range service.Handlers {
		for di := range service.Handlers[hi].CommandDeps {
			dep := &service.Handlers[hi].CommandDeps[di]
			if err := a.normalizeCommandDep(dep, visiting); err != nil {
				return fmt.Errorf("command %q: %w", dep.Command, err)
			}
		}
	}

	if err := a.SetService(*service); err != nil {
		return err
	}
	return nil
}

func (a *SdsService) normalizeCommandDep(dep *CommandDep, visiting map[string]bool) error {
	if err := ValidateCommandDep(*dep); err != nil {
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

func (a *SdsService) normalizeDepTarget(target *DepTarget, visiting map[string]bool) error {
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

func (a *SdsService) validateDepRefs() error {
	for _, service := range a.Services {
		for _, handler := range service.Handlers {
			for _, dep := range handler.CommandDeps {
				for _, target := range dep.Proxies {
					if err := a.validateDepRef(target); err != nil {
						return fmt.Errorf("service %q command %q proxy: %w", service.Name, dep.Command, err)
					}
				}
				for _, target := range dep.Extensions {
					if err := a.validateDepRef(target); err != nil {
						return fmt.Errorf("service %q command %q extension: %w", service.Name, dep.Command, err)
					}
				}
			}
		}
	}
	return nil
}

func (a *SdsService) validateDepRef(target DepTarget) error {
	name := target.Name()
	if name == "" {
		return fmt.Errorf("dep target name is empty")
	}
	if _, err := a.GetService(name); err != nil {
		return fmt.Errorf("service %q not found: %w", name, err)
	}
	return nil
}

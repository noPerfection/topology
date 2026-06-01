package runtime

import config "github.com/noPerfection/context/config"

// Interface is implemented by the dependency runtime.
//
// It doesn't have the `Stop` command.
// Because, stopping must be done by the remote call from other services.
type Interface interface {
	// AddService registers a service in the runtime configuration.
	AddService(target config.DepTarget) error

	// SetService updates an existing service in the runtime configuration.
	SetService(service config.Service) error

	// RemoveService removes a service from the runtime configuration.
	RemoveService(serviceName string) error

	// StartService starts the dependency service with the given parent.
	StartService(serviceName string, optionalParent ...*ParentClient) (string, error)

	// IsServiceRunning checks is the service running or not.
	IsServiceRunning(serviceName string) (bool, error)

	// StopService stops the given dependency service.
	StopService(serviceName string) error
}

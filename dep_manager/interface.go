package dep_manager

import (
	clientConfig "github.com/sds-framework/client-lib/config"
)

// The Interface of the dependency manager.
//
// It doesn't have the `Stop` command.
// Because, stopping must be done by the remote call from other services.
type Interface interface {
	// Run the dependency with the given id and parent.
	Run(dep *Dep, id string, optionalParent ...*clientConfig.Client) error

	// Running checks is the service running or not
	Running(*clientConfig.Client) (bool, error)

	// Close the given dependency service
	Close(c *clientConfig.Client) error
}

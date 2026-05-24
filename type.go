// Package config defines the specific parameters of the Contexts and Dev Context
package context

type ContextType = string

const (
	// DevContext indicates that all dependency proxies are in the local machine
	DevContext ContextType = "development"
	// UnknownContext indicates that the context is unspecified.
	UnknownContext ContextType = "unknown"

	ContextFlag = "context"
	ConfigFlag  = "config"
)

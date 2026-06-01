package config

import "fmt"

// Type defines the available kind of services
// If you are creating a new service, then define the constant value here.
type Type string

// HandlerType defines the available kind of service handlers.
// If you are creating a new handler type, then define the constant value here.
type HandlerType string

const (
	// ProxyType services are handling the message and redirects it to another service
	ProxyType Type = "Proxy"
	// ExtensionType services are exposing the API to be used by Independent, Proxy or other Extension.
	ExtensionType Type = "Extension"
	// IndependentType service means the service is intended to be used as a standalone service
	IndependentType Type = "Independent"

	// SyncReplierType handlers reply to synchronous requests.
	SyncReplierType HandlerType = "SyncReplier"
	// ReplierType handlers reply to requests.
	ReplierType HandlerType = "Replier"
	// PublisherType handlers publish messages.
	PublisherType HandlerType = "Publisher"
	// PairType handlers communicate as a pair socket.
	PairType   HandlerType = "Pair"
	WorkerType HandlerType = "Worker"
)

// ValidateServiceType checks whether the given string is the valid or not.
// If not valid, then returns the error otherwise returns nil.
func ValidateServiceType(t Type) error {
	if t == ProxyType || t == ExtensionType || t == IndependentType {
		return nil
	}

	return fmt.Errorf("'%s' is not valid service type", t)
}

// ValidateHandlerType checks whether the given handler type is valid.
// If not valid, then returns the error otherwise returns nil.
func ValidateHandlerType(t HandlerType) error {
	if t == SyncReplierType || t == ReplierType || t == PublisherType || t == PairType || t == WorkerType {
		return nil
	}

	return fmt.Errorf("'%s' is not valid handler type", t)
}

func (s Type) String() string {
	return string(s)
}

func (h HandlerType) String() string {
	return string(h)
}

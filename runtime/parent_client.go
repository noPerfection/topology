package runtime

import "github.com/noPerfection/protocol/client"

// ParentClient describes the service that a dependency should connect back to.
type ParentClient struct {
	ServiceUrl string             `json:"ServiceUrl"`
	Id         string             `json:"Id"`
	Port       uint64             `json:"Port"`
	TargetType client.HandlerType `json:"TargetType,omitempty"`
}

package context

import "github.com/sds-framework/dev-lib/dep_client"

type Interface interface {
	StartRuntimeHandler() error
	Runtime() dep_client.Interface
}

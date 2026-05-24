package dep_handler

import (
	"fmt"
	"testing"
	"time"

	"github.com/sds-framework/client-lib"
	clientConfig "github.com/sds-framework/client-lib/config"
	"github.com/sds-framework/dev-lib/runtime"
	handlerConfig "github.com/sds-framework/handler-lib/config"
	"github.com/sds-framework/handler-lib/manager_client"
	"github.com/sds-framework/log-lib"
	"github.com/sds-framework/os-lib/path"

	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestDepHandlerSuite struct {
	suite.Suite

	logger            *log.Logger
	depHandler        *DepHandler // the manager to test
	depHandlerManager manager_client.Interface
	currentDir        string               // executable to store the binaries and source codes
	url               string               // dependency source code
	id                string               // the id of the dependency
	parent            *clientConfig.Client // the info about the service to which dependency should connect

	client *client.Socket // imitating the service
}

// Make sure that Account is set to five
// before each test
func (test *TestDepHandlerSuite) SetupTest() {
	s := test.Suite.Require

	logger, _ := log.New("test", false)
	test.logger = logger

	currentDir, err := path.CurrentDir()
	s().NoError(err)
	test.currentDir = currentDir

	srcPath := path.AbsDir(currentDir, "_sds/src")
	binPath := path.AbsDir(currentDir, "_sds/bin")

	// Make sure that the folders don't exist. They will be added later
	depRuntime := runtime.New()
	err = depRuntime.SetPaths(srcPath, binPath)
	s().NoError(err)

	test.depHandler, err = New(depRuntime)
	s().NoError(err)

	// Start the handler
	s().NoError(test.depHandler.Start())

	test.depHandlerManager, err = manager_client.New(ServiceConfig())
	s().NoError(err)

	// wait a bit for closing
	time.Sleep(time.Millisecond * 100)

	// A valid source code that we want to download
	test.url = "github.com/sds-framework/test-manager"

	test.id = "test-manager"
	test.parent = &clientConfig.Client{
		ServiceUrl: "dev-lib",
		Id:         "parent",
		Port:       120,
		TargetType: handlerConfig.SocketType(handlerConfig.ReplierType),
	}

	config := ServiceConfig()
	socketType := handlerConfig.SocketType(config.Type)
	socket, err := client.NewRaw(socketType, fmt.Sprintf("inproc://%s", config.Id))
	s().NoError(err)

	test.client = socket
	test.client.Timeout(time.Second * 30)
	test.client.Attempt(1)
}

func (test *TestDepHandlerSuite) TearDownTest() {
	s := test.Suite.Require

	s().NoError(test.client.Close())

	s().NoError(test.depHandlerManager.Close())

	// Wait a bit for the close of the handler thread.
	time.Sleep(time.Millisecond * 100)
}

//
//// Test_13_Start tests DepRunning, RunDep and CloseDep commands.
//func (test *TestDepHandlerSuite) Test_13_Start() {
//	s := test.Suite.Require
//
//	depClient := &clientConfig.Client{
//		ServiceUrl: test.url,
//		Id:         test.id,
//		Port:       6000,
//		TargetType: handlerConfig.SocketType(handlerConfig.ReplierType),
//	}
//
//	src, err := source.New(test.url)
//	s().NoError(err)
//	src.SetBranch("server") // the sample server is written in this branch.
//
//	// Let's run it
//	runReq := message.Request{
//		Command: RunDep,
//		Parameters: key_value.New().
//			Set("parent", test.parent).
//			Set("url", src.Url).
//			Set("id", test.id),
//	}
//	rep, err = test.client.Request(&runReq)
//	s().NoError(err)
//	s().True(rep.IsOK())
//
//	// Just wait a bit for initialization of the service
//	time.Sleep(time.Millisecond * 100)
//
//	// check that service is running
//	runningReq := message.Request{
//		Command: DepRunning,
//		Parameters: key_value.New().
//			Set("dep", depClient),
//	}
//	running, err := test.client.Request(&runningReq)
//	s().NoError(err)
//	s().True(running.IsOK())
//	result, err := running.ReplyParameters().BoolValue("running")
//	s().NoError(err)
//	s().True(result)
//
//	// Close the service
//	closeReq := message.Request{
//		Command: CloseDep,
//		Parameters: key_value.New().
//			Set("dep", depClient),
//	}
//	running, err = test.client.Request(&closeReq)
//	s().NoError(err)
//	s().True(running.IsOK())
//
//	// Wait a bit for closing the source process
//	time.Sleep(time.Millisecond * 100)
//
//	// Checking for a running source after it was closed must fail
//	running, err = test.client.Request(&runningReq)
//	s().NoError(err)
//	s().True(running.IsOK())
//	result, err = running.ReplyParameters().BoolValue("running")
//	s().NoError(err)
//	s().False(result)
//
//}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDepManager(t *testing.T) {
	suite.Run(t, new(TestDepHandlerSuite))
}

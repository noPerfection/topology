package dep_client

import (
	"testing"
	"time"

	clientConfig "github.com/sds-framework/client-lib/config"
	"github.com/sds-framework/dev-lib/dep_handler"
	"github.com/sds-framework/dev-lib/dep_manager"
	handlerConfig "github.com/sds-framework/handler-lib/config"
	"github.com/sds-framework/handler-lib/manager_client"
	"github.com/sds-framework/log-lib"
	"github.com/sds-framework/os-lib/path"

	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestDepClientSuite struct {
	suite.Suite

	logger            *log.Logger
	depHandler        *dep_handler.DepHandler // the manager to test
	depHandlerManager manager_client.Interface
	currentDir        string               // executable to store the binaries and source codes
	url               string               // dependency source code
	id                string               // the id of the dependency
	parent            *clientConfig.Client // the info about the service to which dependency should connect

	client *Client
}

// Make sure that Account is set to five
// before each test
func (test *TestDepClientSuite) SetupTest() {
	s := test.Suite.Require

	logger, _ := log.New("test", false)
	test.logger = logger

	currentDir, err := path.CurrentDir()
	s().NoError(err)
	test.currentDir = currentDir

	srcPath := path.AbsDir(currentDir, "_sds/src")
	binPath := path.AbsDir(currentDir, "_sds/bin")

	// Make sure that the folders don't exist. They will be added later
	manager := dep_manager.New()
	err = manager.SetPaths(srcPath, binPath)
	s().NoError(err)

	test.depHandler, err = dep_handler.New(manager)
	s().NoError(err)

	// Start the handler
	s().NoError(test.depHandler.Start())

	test.depHandlerManager, err = manager_client.New(dep_handler.ServiceConfig())
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

	socket, err := New()
	s().NoError(err)

	test.client = socket
	test.client.Timeout(time.Second * 30)
	test.client.Attempt(1)
}

func (test *TestDepClientSuite) TearDownTest() {
	s := test.Suite.Require

	s().NoError(test.client.Close())

	s().NoError(test.depHandlerManager.Close())

	// Wait a bit for the close of the handler thread.
	time.Sleep(time.Millisecond * 100)
}

// Test_10_Installed checks DepInstalled.
func (test *TestDepClientSuite) Test_10_Installed() {
	s := test.Suite.Require

	installed, err := test.client.Installed(test.url, "")
	s().NoError(err)
	s().False(installed)
}

// Test_11_Uninstall deletes the dependency binary and source code when present.
func (test *TestDepClientSuite) Test_11_Uninstall() {
	s := test.Suite.Require

	installed, err := test.client.Installed(test.url, "")
	s().NoError(err)
	s().False(installed)

	// Uninstall
	err = test.client.Uninstall(test.url, "", "")
	s().NoError(err)

	// wait a bit for effect
	time.Sleep(time.Millisecond * 100)

	// After uninstallation, we should not have the binary
	installed, err = test.client.Installed(test.url, "")
	s().NoError(err)
	s().False(installed)
}

//// Test_13_Run tests DepRunning, RunDep and CloseDep commands.
//func (test *TestDepClientSuite) Test_13_Run() {
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
//	// Let's run the dependency
//	test.logger.Info("request to run the dependency", "srcUrl", src.Url, "id", test.id)
//	err = test.client.Run(src.Url, test.id, test.parent)
//	s().NoError(err)
//
//	// Just wait a bit for initialization of the service
//	time.Sleep(time.Millisecond * 100)
//
//	// check that service is running
//	test.logger.Info("check dependency status")
//	running, err := test.client.IsRunning(depClient)
//	s().NoError(err)
//	s().True(running)
//	test.logger.Info("status returned from dependency manager", "running", running, "error", err)
//
//	// CloseDep the service
//	test.logger.Info("send a signal to close dependency")
//
//	err = test.client.CloseDep(depClient)
//	s().NoError(err)
//
//	// Wait a bit for closing the source process
//	time.Sleep(time.Millisecond * 100)
//
//	// Checking for a running source after it was closed must fail
//	test.logger.Info("check again the dependency status")
//	running, err = test.client.IsRunning(depClient)
//	test.logger.Info("closed dependency status returned", "running", running, "error", err)
//	s().NoError(err)
//	s().False(running)
//
//	test.logger.Info("uninstall the dependency")
//
//	// Clean out the installed files
//	err = test.client.Uninstall(src)
//	s().NoError(err)
//}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDepClient(t *testing.T) {
	suite.Run(t, new(TestDepClientSuite))
}

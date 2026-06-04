package topology

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/control"
	"github.com/noPerfection/protocol/message"
	config "github.com/noPerfection/topology/config"

	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestHandlerSuite struct {
	suite.Suite

	logger            *log.Logger
	depHandler        *Handler // the manager to test
	depHandlerManager *sync_replier.Client
	url               string // dependency source code
	id                string // the id of the dependency
	parent            string // the service name that dependency should connect back to

	client *client.Socket // imitating the service
}

// Make sure that Account is set to five
// before each test
func (test *TestHandlerSuite) SetupTest() {
	s := test.Suite.Require

	logger, _ := log.New("test", false)
	test.logger = logger

	var err error
	test.depHandler, err = newHandler(&config.NoPerfection{})
	s().NoError(err)

	// Start the handler
	s().NoError(test.depHandler.Start())

	controlConfig := control.CreateInternalConfig(HandlerConfig())
	test.depHandlerManager, err = sync_replier.NewClient(controlConfig.Id, controlConfig.Port)
	s().NoError(err)

	// wait a bit for closing
	time.Sleep(time.Millisecond * 100)

	// A valid source code that we want to download
	test.url = "github.com/noPerfection/test-manager"

	test.id = "test-manager"
	test.parent = "parent"

	handlerCfg := HandlerConfig()
	socket, err := client.New(handlerCfg.Id, handlerCfg.Port, client.HandlerType(handlerCfg.Type))
	s().NoError(err)

	test.client = socket
	test.client.Timeout(time.Second * 30)
	test.client.Attempt(1)
}

func (test *TestHandlerSuite) TearDownTest() {
	s := test.Suite.Require

	s().NoError(test.client.Close())

	_, err := test.depHandlerManager.Request(&message.Request{
		Command:    control.HandlerClose,
		Parameters: datatype.New(),
	})
	s().NoError(err)
	s().NoError(test.depHandlerManager.Close())

	// Wait a bit for the close of the handler thread.
	time.Sleep(time.Millisecond * 100)
}

func TestHandlerTopologyInterfaceBeforeStart(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "app.json")
	appConfig, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	handler, err := newHandler(&appConfig)
	if err != nil {
		t.Fatalf("newHandler: %v", err)
	}

	err = handler.AddService(config.NewServiceRecord(config.Service{
		Type: config.ProxyType,
		Name: "pre-start-service",
		Handlers: []config.Handler{
			{
				Type:     config.ReplierType,
				Category: ServiceManagerCategory,
				Endpoint: message.NewEndpoint("pre-start-manager", 6100),
			},
		},
	}))
	if err != nil {
		t.Fatalf("handler.AddService before start: %v", err)
	}

	if _, err := appConfig.GetService("pre-start-service"); err != nil {
		t.Fatalf("appConfig.GetService: %v", err)
	}
}

func (test *TestHandlerSuite) TestTopologyInterfaceAfterStartBlocked() {
	s := test.Require

	s().Error(test.depHandler.AddService(config.NewServiceRecord(config.Service{Name: "blocked", Type: config.ProxyType})))
	s().Error(test.depHandler.SetService(config.NewServiceRecord(config.Service{Name: "blocked", Type: config.ProxyType})))
	s().Error(test.depHandler.RemoveService("blocked"))
	_, err := test.depHandler.StartService("blocked")
	s().Error(err)
	_, err = test.depHandler.IsServiceRunning("blocked")
	s().Error(err)
	s().Error(test.depHandler.StopService("blocked"))
}

//
//// Test_13_Start tests IsServiceRunning, StartService and StopService commands.
//func (test *TestHandlerSuite) Test_13_Start() {
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
//		Command: StartService,
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
//		Command: IsServiceRunning,
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
//		Command: StopService,
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
func TestHandler(t *testing.T) {
	suite.Run(t, new(TestHandlerSuite))
}

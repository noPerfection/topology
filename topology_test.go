package topology

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/noPerfection/log"
	"github.com/noPerfection/os/path"
	"github.com/noPerfection/protocol/message"
	config "github.com/noPerfection/topology/config"
	"github.com/stretchr/testify/suite"
)

// todo for public functions test with the nil values

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestDepManagerSuite struct {
	suite.Suite

	logger       *log.Logger
	topology     *Topology     // the topology to test
	currentDir   string        // executable to store the binaries and source codes
	url          string        // dependency source code
	id           string        // the id of the dependency
	parent       *ParentClient // the info about the service to which dependency should connect
	localTestDir string
}

func (test *TestDepManagerSuite) setServiceStartCommand(name string, startCommand string) {
	for i := range test.topology.config.Services {
		if test.topology.config.Services[i].Name == name {
			test.topology.config.Services[i].StartCommand = startCommand
			return
		}
	}

	test.topology.config.Services = append(test.topology.config.Services, config.Service{
		Name:         name,
		StartCommand: startCommand,
	})
}

func (test *TestDepManagerSuite) requireTestBinary(binary string) {
	_, err := os.Stat(binary)
	if os.IsNotExist(err) {
		test.T().Skipf("test service binary %q is missing; build the _test_services fixtures to run this test", binary)
	}
	test.Require().NoError(err)
}

// Make sure that Account is set to five
// before each test
func (test *TestDepManagerSuite) SetupTest() {
	s := test.Require

	logger, _ := log.New("TestDepManagerSuite", false)
	test.logger = logger

	currentDir, err := path.CurrentDir()
	s().NoError(err)
	test.currentDir = currentDir

	test.topology = &Topology{
		config: &config.NoPerfection{
			Services: []config.Service{
				{
					Type:         config.ProxyType,
					Name:         "test-manager",
					StartCommand: "test",
					Handlers: []config.Handler{
						{
							Type:     config.ReplierType,
							Category: ManagerHandlerCategory,
							Endpoint: message.NewEndpoint("test-manager", 6000),
						},
					},
				},
			},
		},
		sameServices:     make(map[string]int),
		runningProcesses: make(map[string]*Process, 0),
		timeout:          DefaultTimeout,
	}

	// A valid source code that we want to download
	test.url = "github.com/noPerfection/test-manager"

	test.id = "test-manager"
	test.parent = &ParentClient{
		ServiceUrl: "topology",
		Id:         "parent",
		Port:       120,
	}

	test.localTestDir = filepath.Join("_test_services")
}

// Test_0_New tests the creation of the Topology.
func (test *TestDepManagerSuite) Test_0_New() {
	s := test.Require

	cfg := &config.NoPerfection{}
	depTopology := New(cfg)
	s().NotNil(depTopology)
	s().Same(cfg, depTopology.config)
	s().NotNil(depTopology.sameServices)
	s().NotNil(depTopology.runningProcesses)
	s().Equal(DefaultTimeout, depTopology.timeout)

	test.topology = depTopology
}

func (test *TestDepManagerSuite) Test_10_GenerateId() {
	s := test.Require

	id, err := test.topology.GenerateId(test.id)
	s().NoError(err)
	s().Equal("test-manager1", id)
	s().Equal(1, test.topology.sameServices[test.id])

	test.topology.runningProcesses[id] = &Process{
		config: &test.topology.config.Services[0],
		id:     id,
	}

	id, err = test.topology.GenerateId(test.id)
	s().NoError(err)
	s().Equal("test-manager2", id)
	s().Equal(2, test.topology.sameServices[test.id])

	delete(test.topology.runningProcesses, "test-manager1")
	test.topology.refreshServiceCount(test.id)
	s().Equal(0, test.topology.sameServices[test.id])
}

func (test *TestDepManagerSuite) Test_12_ServiceConfig() {
	s := test.Require

	cfgPath := filepath.Join(test.T().TempDir(), "app.json")
	cfg, err := config.Load(cfgPath)
	s().NoError(err)
	test.topology = New(&cfg)

	service := config.Service{
		Type:         config.ProxyType,
		Name:         "extra-service",
		StartCommand: "echo extra",
		Handlers: []config.Handler{
			{
				Type:     config.ReplierType,
				Category: ManagerHandlerCategory,
				Endpoint: message.NewEndpoint("extra-service-manager", 6001),
			},
		},
	}
	err = test.topology.AddService(config.InlineTarget(service))
	s().NoError(err)

	got, err := test.topology.config.GetService("extra-service")
	s().NoError(err)
	s().Equal("echo extra", got.StartCommand)

	err = test.topology.RemoveService("extra-service")
	s().NoError(err)

	_, err = test.topology.config.GetService("extra-service")
	s().Error(err)

	err = test.topology.RemoveService("missing")
	s().Error(err)

	err = test.topology.AddService(config.InlineTarget(config.Service{
		Type:         config.ProxyType,
		Name:         "plain-service",
		StartCommand: "echo plain",
	}))
	s().NoError(err)
	err = test.topology.RemoveService("plain-service")
	s().Error(err)
}

func (test *TestDepManagerSuite) Test_13_AddServiceTargetValidation() {
	s := test.Require

	cfgPath := filepath.Join(test.T().TempDir(), "app.json")
	cfg, err := config.Load(cfgPath)
	s().NoError(err)
	s().NoError(cfg.SetService(test.topology.config.Services[0]))
	test.topology = New(&cfg)

	err = test.topology.AddService(config.RefTarget("test-manager"))
	s().NoError(err)

	err = test.topology.AddService(config.RefTarget("missing-service"))
	s().Error(err)

	err = test.topology.AddService(config.InlineTarget(config.Service{
		Type: config.ProxyType,
		Name: "duplicate-socket",
		Handlers: []config.Handler{
			{
				Type:     config.ReplierType,
				Category: ManagerHandlerCategory,
				Endpoint: message.NewEndpoint("test-manager", 6000),
			},
		},
	}))
	s().Error(err)

	err = test.topology.AddService(config.InlineTarget(config.Service{
		Type: config.ProxyType,
		Name: "nested-parent",
		Handlers: []config.Handler{
			{
				Type:     config.ReplierType,
				Category: ManagerHandlerCategory,
				Endpoint: message.NewEndpoint("nested-parent-manager", 6100),
				CommandDeps: []config.DepService{
					{
						Name: "proxy",
						Proxies: []config.DepTarget{
							config.InlineTarget(config.Service{
								Type: config.ProxyType,
								Name: "nested-child",
								Handlers: []config.Handler{
									{
										Type:     config.ReplierType,
										Category: ManagerHandlerCategory,
										Endpoint: message.NewEndpoint("nested-child-manager", 6101),
									},
								},
							}),
						},
					},
				},
			},
		},
	}))
	s().NoError(err)

	_, err = test.topology.config.GetService("nested-parent")
	s().NoError(err)
	_, err = test.topology.config.GetService("nested-child")
	s().NoError(err)

	err = test.topology.AddService(config.InlineTarget(config.Service{
		Type: config.ProxyType,
		Name: "service-level-parent",
		HandlerDeps: []config.DepService{
			{
				Name: "manager",
				Proxies: []config.DepTarget{
					config.InlineTarget(config.Service{
						Type: config.ProxyType,
						Name: "service-level-child",
						Handlers: []config.Handler{
							{
								Type:     config.ReplierType,
								Category: ManagerHandlerCategory,
								Endpoint: message.NewEndpoint("service-level-child-manager", 6201),
							},
						},
					}),
				},
			},
		},
		Handlers: []config.Handler{
			{
				Type:     config.ReplierType,
				Category: ManagerHandlerCategory,
				Endpoint: message.NewEndpoint("service-level-parent-manager", 6200),
			},
		},
	}))
	s().NoError(err)

	_, err = test.topology.config.GetService("service-level-parent")
	s().NoError(err)
	_, err = test.topology.config.GetService("service-level-child")
	s().NoError(err)
}

// Test_20_Run runs the given binary.
func (test *TestDepManagerSuite) Test_20_Run() {
	s := test.Require

	localBin := path.BinPath(filepath.Join(test.localTestDir, "test-manager", "bin"), "test")
	invalidBin := path.BinPath(filepath.Join(test.localTestDir, "test-manager", "bin"), "non_existing")
	test.requireTestBinary(localBin)
	test.setServiceStartCommand(test.id, localBin)

	_, ok := test.topology.runningProcesses[test.id+"1"]
	s().False(ok)

	// running nil values must exist
	var depTopology *Topology
	_, err := depTopology.StartService(test.id, test.parent)
	s().Error(err)

	_, err = test.topology.StartService("", test.parent)
	s().Error(err) // missing service name
	_, err = test.topology.StartService(test.id, nil)
	s().Error(err) // missing parent

	test.setServiceStartCommand("no-command", "")
	_, err = test.topology.StartService("no-command", test.parent)
	s().Error(err) // no start command

	// the binary doesn't exist
	test.setServiceStartCommand(test.id, invalidBin)
	_, err = test.topology.StartService(test.id, test.parent)
	s().Error(err) // no binary

	// Let's run it, it should exit immediately
	test.setServiceStartCommand(test.id, localBin)
	id, err := test.topology.StartService(test.id, test.parent)
	s().NoError(err)

	_, ok = test.topology.runningProcesses[id]
	s().True(ok)

	// clean out
	_, ok = test.topology.runningProcesses[id]
	if ok {
		onStop := test.topology.OnStop(id)
		err = <-onStop
		s().NoError(err)

		_, running := test.topology.runningProcesses[id]
		s().False(running)
	}
}

// Test_21_RunError runs the binary that exits with error.
// If it exists with an error, it must catch it.
func (test *TestDepManagerSuite) Test_21_RunError() {
	s := test.Require

	localBin := path.BinPath(filepath.Join(test.localTestDir, "with-error", "bin"), "test")
	test.requireTestBinary(localBin)
	test.setServiceStartCommand(test.id, localBin)

	// Let's run it
	id, err := test.topology.StartService(test.id, test.parent)
	s().NoError(err)

	// make sure that it exists
	_, ok := test.topology.runningProcesses[id]
	s().True(ok)

	stopChan := test.topology.OnStop(id)
	s().NotNil(stopChan)

	err = <-stopChan
	s().Error(err)

	// the closed service is removed from Topology
	_, ok = test.topology.runningProcesses[id]
	s().False(ok)

}

// Test_22_Running checks that service is running
func (test *TestDepManagerSuite) Test_22_Running() {
	s := test.Require

	localBin := path.BinPath(filepath.Join(test.localTestDir, "server", "bin"), "test")
	test.requireTestBinary(localBin)
	test.setServiceStartCommand(test.id, localBin)

	// First, install the manager
	// Let's run it
	id, err := test.topology.StartService(test.id, test.parent)
	s().NoError(err)
	s().NotNil(test.topology.runningProcesses[id]) // cmd == nil indicates that the program was closed

	// Check is the service running
	running, err := test.topology.IsServiceRunning(test.id)
	s().NoError(err)
	s().True(running)

	// service is running two seconds. after that running should return false
	onStop := test.topology.OnStop(id)
	s().NotNil(onStop)
	err = <-onStop
	s().NoError(err)

	s().Nil(test.topology.runningProcesses[id]) // cmd == nil indicates that the program was closed
	running, err = test.topology.IsServiceRunning(test.id)
	s().NoError(err)
	s().False(running)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDepManager(t *testing.T) {
	suite.Run(t, new(TestDepManagerSuite))
}

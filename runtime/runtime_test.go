package runtime

import (
	"os"
	"path/filepath"
	"testing"

	config "github.com/noPerfection/context/config"
	"github.com/noPerfection/log"
	"github.com/noPerfection/os/path"
	"github.com/stretchr/testify/suite"
)

// todo for public functions test with the nil values

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestDepManagerSuite struct {
	suite.Suite

	logger       *log.Logger
	runtime      *Runtime      // the runtime to test
	currentDir   string        // executable to store the binaries and source codes
	url          string        // dependency source code
	id           string        // the id of the dependency
	parent       *ParentClient // the info about the service to which dependency should connect
	localTestDir string
}

func (test *TestDepManagerSuite) setServiceStartCommand(name string, startCommand string) {
	for i := range test.runtime.config.Services {
		if test.runtime.config.Services[i].Name == name {
			test.runtime.config.Services[i].StartCommand = startCommand
			return
		}
	}

	test.runtime.config.Services = append(test.runtime.config.Services, config.Service{
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

	test.runtime = &Runtime{
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
							Socket: config.Socket{
								Id:   "test-manager",
								Port: 6000,
							},
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
		ServiceUrl: "context",
		Id:         "parent",
		Port:       120,
	}

	test.localTestDir = filepath.Join("../_test_services")
}

// Test_0_New tests the creation of the Runtime.
func (test *TestDepManagerSuite) Test_0_New() {
	s := test.Require

	cfg := &config.NoPerfection{}
	depRuntime := New(cfg)
	s().NotNil(depRuntime)
	s().Same(cfg, depRuntime.config)
	s().NotNil(depRuntime.sameServices)
	s().NotNil(depRuntime.runningProcesses)
	s().Equal(DefaultTimeout, depRuntime.timeout)

	test.runtime = depRuntime
}

func (test *TestDepManagerSuite) Test_10_GenerateId() {
	s := test.Require

	id, err := test.runtime.GenerateId(test.id)
	s().NoError(err)
	s().Equal("test-manager1", id)
	s().Equal(1, test.runtime.sameServices[test.id])

	test.runtime.runningProcesses[id] = &Process{
		config: &test.runtime.config.Services[0],
		id:     id,
	}

	id, err = test.runtime.GenerateId(test.id)
	s().NoError(err)
	s().Equal("test-manager2", id)
	s().Equal(2, test.runtime.sameServices[test.id])

	delete(test.runtime.runningProcesses, "test-manager1")
	test.runtime.refreshServiceCount(test.id)
	s().Equal(0, test.runtime.sameServices[test.id])
}

func (test *TestDepManagerSuite) Test_12_ServiceConfig() {
	s := test.Require

	cfgPath := filepath.Join(test.T().TempDir(), "app.json")
	cfg, err := config.Load(cfgPath)
	s().NoError(err)
	test.runtime = New(&cfg)

	service := config.Service{
		Type:         config.ProxyType,
		Name:         "extra-service",
		StartCommand: "echo extra",
		Handlers: []config.Handler{
			{
				Type:     config.ReplierType,
				Category: ManagerHandlerCategory,
				Socket: config.Socket{
					Id:   "extra-service-manager",
					Port: 6001,
				},
			},
		},
	}
	err = test.runtime.AddService(config.InlineTarget(service))
	s().NoError(err)

	got, err := test.runtime.config.GetService("extra-service")
	s().NoError(err)
	s().Equal("echo extra", got.StartCommand)

	err = test.runtime.RemoveService("extra-service")
	s().NoError(err)

	_, err = test.runtime.config.GetService("extra-service")
	s().Error(err)

	err = test.runtime.RemoveService("missing")
	s().Error(err)

	err = test.runtime.AddService(config.InlineTarget(config.Service{
		Type:         config.ProxyType,
		Name:         "plain-service",
		StartCommand: "echo plain",
	}))
	s().NoError(err)
	err = test.runtime.RemoveService("plain-service")
	s().Error(err)
}

func (test *TestDepManagerSuite) Test_13_AddServiceTargetValidation() {
	s := test.Require

	cfgPath := filepath.Join(test.T().TempDir(), "app.json")
	cfg, err := config.Load(cfgPath)
	s().NoError(err)
	s().NoError(cfg.SetService(test.runtime.config.Services[0]))
	test.runtime = New(&cfg)

	err = test.runtime.AddService(config.RefTarget("test-manager"))
	s().NoError(err)

	err = test.runtime.AddService(config.RefTarget("missing-service"))
	s().Error(err)

	err = test.runtime.AddService(config.InlineTarget(config.Service{
		Type: config.ProxyType,
		Name: "duplicate-socket",
		Handlers: []config.Handler{
			{
				Type:     config.ReplierType,
				Category: ManagerHandlerCategory,
				Socket:   config.Socket{Id: "test-manager", Port: 6000},
			},
		},
	}))
	s().Error(err)

	err = test.runtime.AddService(config.InlineTarget(config.Service{
		Type: config.ProxyType,
		Name: "nested-parent",
		Handlers: []config.Handler{
			{
				Type:     config.ReplierType,
				Category: ManagerHandlerCategory,
				Socket:   config.Socket{Id: "nested-parent-manager", Port: 6100},
				CommandDeps: []config.CommandDep{
					{
						Command: "proxy",
						Proxies: []config.DepTarget{
							config.InlineTarget(config.Service{
								Type: config.ProxyType,
								Name: "nested-child",
								Handlers: []config.Handler{
									{
										Type:     config.ReplierType,
										Category: ManagerHandlerCategory,
										Socket:   config.Socket{Id: "nested-child-manager", Port: 6101},
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

	_, err = test.runtime.config.GetService("nested-parent")
	s().NoError(err)
	_, err = test.runtime.config.GetService("nested-child")
	s().NoError(err)
}

// Test_20_Run runs the given binary.
func (test *TestDepManagerSuite) Test_20_Run() {
	s := test.Require

	localBin := path.BinPath(filepath.Join(test.localTestDir, "test-manager", "bin"), "test")
	invalidBin := path.BinPath(filepath.Join(test.localTestDir, "test-manager", "bin"), "non_existing")
	test.requireTestBinary(localBin)
	test.setServiceStartCommand(test.id, localBin)

	_, ok := test.runtime.runningProcesses[test.id+"1"]
	s().False(ok)

	// running nil values must exist
	var depRuntime *Runtime
	_, err := depRuntime.StartService(test.id, test.parent)
	s().Error(err)

	_, err = test.runtime.StartService("", test.parent)
	s().Error(err) // missing service name
	_, err = test.runtime.StartService(test.id, nil)
	s().Error(err) // missing parent

	test.setServiceStartCommand("no-command", "")
	_, err = test.runtime.StartService("no-command", test.parent)
	s().Error(err) // no start command

	// the binary doesn't exist
	test.setServiceStartCommand(test.id, invalidBin)
	_, err = test.runtime.StartService(test.id, test.parent)
	s().Error(err) // no binary

	// Let's run it, it should exit immediately
	test.setServiceStartCommand(test.id, localBin)
	id, err := test.runtime.StartService(test.id, test.parent)
	s().NoError(err)

	_, ok = test.runtime.runningProcesses[id]
	s().True(ok)

	// clean out
	_, ok = test.runtime.runningProcesses[id]
	if ok {
		onStop := test.runtime.OnStop(id)
		err = <-onStop
		s().NoError(err)

		_, running := test.runtime.runningProcesses[id]
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
	id, err := test.runtime.StartService(test.id, test.parent)
	s().NoError(err)

	// make sure that it exists
	_, ok := test.runtime.runningProcesses[id]
	s().True(ok)

	stopChan := test.runtime.OnStop(id)
	s().NotNil(stopChan)

	err = <-stopChan
	s().Error(err)

	// the closed service is removed from Runtime
	_, ok = test.runtime.runningProcesses[id]
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
	id, err := test.runtime.StartService(test.id, test.parent)
	s().NoError(err)
	s().NotNil(test.runtime.runningProcesses[id]) // cmd == nil indicates that the program was closed

	// Check is the service running
	running, err := test.runtime.IsServiceRunning(test.id)
	s().NoError(err)
	s().True(running)

	// service is running two seconds. after that running should return false
	onStop := test.runtime.OnStop(id)
	s().NotNil(onStop)
	err = <-onStop
	s().NoError(err)

	s().Nil(test.runtime.runningProcesses[id]) // cmd == nil indicates that the program was closed
	running, err = test.runtime.IsServiceRunning(test.id)
	s().NoError(err)
	s().False(running)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDepManager(t *testing.T) {
	suite.Run(t, new(TestDepManagerSuite))
}

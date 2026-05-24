package dep_manager

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pebbe/zmq4"
	clientConfig "github.com/sds-framework/client-lib/config"
	"github.com/sds-framework/log-lib"
	"github.com/sds-framework/os-lib/path"
	"github.com/stretchr/testify/suite"
)

// todo for public functions test with the nil values

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestDepManagerSuite struct {
	suite.Suite

	logger       *log.Logger
	depManager   *DepManager          // the manager to test
	currentDir   string               // executable to store the binaries and source codes
	url          string               // dependency source code
	id           string               // the id of the dependency
	parent       *clientConfig.Client // the info about the service to which dependency should connect
	localTestDir string
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

	srcPath := path.AbsDir(currentDir, "_sds/src")
	binPath := path.AbsDir(currentDir, "_sds/bin")

	// Make sure that the folders don't exist. They will be added later
	test.depManager = &DepManager{
		Src:         srcPath,
		Bin:         binPath,
		runningDeps: make(map[string]*Dep, 0),
		timeout:     DefaultTimeout,
	}

	// A valid source code that we want to download
	test.url = "github.com/sds-framework/test-manager"

	test.id = "test-manager"
	test.parent = &clientConfig.Client{
		ServiceUrl: "dev-lib",
		Id:         "parent",
		Port:       120,
	}

	test.localTestDir = filepath.Join("../_test_services")
}

// Test_0_New tests the creation of the DepManager managers with the DepManager.SetPaths method.
func (test *TestDepManagerSuite) Test_0_New() {
	s := test.Require

	fmt.Printf("Test.New: %s bin is nil dep manager: %v\n", test.depManager.Bin, test.depManager == nil)

	// Before testing, we make sure that the files don't exist
	exist, err := path.DirExist(test.depManager.Bin)
	s().NoError(err)
	s().False(exist)

	exist, err = path.DirExist(test.depManager.Src)
	s().NoError(err)
	s().False(exist)

	// If we create the DepManager manager with 'New,' it will create the folders.
	depManager := New()
	err = depManager.SetPaths(test.depManager.Src, test.depManager.Bin)
	s().NoError(err)

	// Now we can check for the directories
	exist, _ = path.DirExist(depManager.Src)
	s().True(exist)

	exist, _ = path.DirExist(depManager.Bin)
	s().True(exist)

	test.depManager = depManager
}

// Test_12_NewDep tests NewDep function.
func (test *TestDepManagerSuite) Test_12_NewDep() {
	s := test.Require

	localSrcPath := path.AbsDir(test.currentDir, "_localSrc")

	// default dep managed by the DepManager
	dep, err := NewDep(test.url, "", "")
	s().NoError(err)
	s().Len(dep.LocalUrl(), 0)
	s().Zero(dep.binPath)

	// trying to create a dep with the invalid url must fail
	_, err = NewDep("git://invalid_url", "", "")
	s().Error(err)

	// trying with the custom source that doesn't exist
	_, err = NewDep(test.url, localSrcPath, "")
	s().Error(err)

	// with the custom local source path it must be successful
	err = path.MakeDir(localSrcPath)
	s().NoError(err)
	goMod := filepath.Join(localSrcPath, "go.mod")
	goModFile, err := os.Create(goMod)
	s().NoError(err)
	err = goModFile.Close()
	s().NoError(err)

	dep, err = NewDep(test.url, localSrcPath, "")
	s().NoError(err)
	s().NotZero(len(dep.LocalUrl()))
	s().Zero(dep.binPath)

	// new dep with the custom binary
	exist, err := path.DirExist(test.localTestDir)
	s().NoError(err)
	s().True(exist)
	localBin := path.BinPath(filepath.Join(test.localTestDir, "test-manager", "bin"), "test")
	exist, err = path.FileExist(localBin)
	s().NoError(err)
	s().True(exist)

	dep, err = NewDep(test.url, "", localBin)
	s().NoError(err)
	s().Zero(len(dep.LocalUrl()))
	s().Equal(localBin, dep.binPath)

	// clean out the parameters
	err = os.RemoveAll(localSrcPath)
	s().NoError(err)
}

// Test_20_Run runs the given binary.
func (test *TestDepManagerSuite) Test_20_Run() {
	s := test.Require

	localBin := path.BinPath(filepath.Join(test.localTestDir, "test-manager", "bin"), "test")
	invalidBin := path.BinPath(filepath.Join(test.localTestDir, "test-manager", "bin"), "non_existing")

	dep, err := NewDep(test.url, "", localBin)
	s().NoError(err)

	// no running files
	_, ok := test.depManager.runningDeps[test.id]
	s().False(ok)

	// running nil values must exist
	var depManager *DepManager
	err = depManager.Run(dep, test.id, test.parent)
	s().Error(err)

	err = test.depManager.Run(nil, test.id, test.parent)
	s().Error(err) // missing dep
	err = test.depManager.Run(dep, "", test.parent)
	s().Error(err) // missing id
	err = test.depManager.Run(dep, test.id, nil)
	s().Error(err) // missing parent

	noBinDep, err := NewDep(test.url, "", "")
	s().NoError(err)
	err = test.depManager.Run(noBinDep, test.id, test.parent)
	s().Error(err) // no binary

	// the binary doesn't exist
	invalidDep, err := NewDep(test.url, "", localBin)
	s().NoError(err)
	invalidDep.binPath = invalidBin
	err = test.depManager.Run(invalidDep, test.id, test.parent)
	s().Error(err) // no binary

	// Let's run it, it should exit immediately
	err = test.depManager.Run(dep, test.id, test.parent)
	s().NoError(err)

	_, ok = test.depManager.runningDeps[test.id]
	s().True(ok)

	// trying to run again must fail
	err = test.depManager.Run(dep, test.id, test.parent)
	s().Error(err)

	// clean out
	_, ok = test.depManager.runningDeps[test.id]
	if ok {
		onStop := test.depManager.OnStop(test.id)
		err = <-onStop
		s().NoError(err)

		_, running := test.depManager.runningDeps[test.id]
		s().False(running)
	}
}

// Test_21_RunError runs the binary that exits with error.
// If it exists with an error, it must catch it.
func (test *TestDepManagerSuite) Test_21_RunError() {
	s := test.Require

	localBin := path.BinPath(filepath.Join(test.localTestDir, "with-error", "bin"), "test")

	dep, err := NewDep(test.url, "", localBin)
	s().NoError(err)
	dep.SetBranch("error-exit") // this branch intentionally exits the program with an error.

	// First, make sure that developer built the binary
	exist := test.depManager.binExist(dep)
	s().True(exist)

	// Let's run it
	err = test.depManager.Run(dep, test.id, test.parent)
	s().NoError(err)

	// make sure that it exists
	_, ok := test.depManager.runningDeps[test.id]
	s().True(ok)

	stopChan := test.depManager.OnStop(test.id)
	s().NotNil(stopChan)

	err = <-stopChan
	s().Error(err)

	// the closed service is removed from DepManager
	_, ok = test.depManager.runningDeps[test.id]
	s().False(ok)

}

// Test_22_Running checks that service is running
func (test *TestDepManagerSuite) Test_22_Running() {
	s := test.Require

	client := &clientConfig.Client{
		ServiceUrl: "test-manager",
		Id:         test.id,
		Port:       6000,
		TargetType: zmq4.REP,
	}
	localBin := path.BinPath(filepath.Join(test.localTestDir, "server", "bin"), "test")

	dep, err := NewDep(test.url, "", localBin)
	s().NoError(err)
	dep.SetBranch("server") // the sample server is written in this branch.

	// First, install the manager
	// Let's run it
	err = test.depManager.Run(dep, test.id, test.parent)
	s().NoError(err)
	s().NotNil(test.depManager.runningDeps[test.id]) // cmd == nil indicates that the program was closed

	// Check is the service running
	running, err := test.depManager.Running(client)
	s().NoError(err)
	s().True(running)

	// service is running two seconds. after that running should return false
	onStop := test.depManager.OnStop(test.id)
	s().NotNil(onStop)
	err = <-onStop
	s().NoError(err)

	s().Nil(test.depManager.runningDeps[test.id]) // cmd == nil indicates that the program was closed
	running, err = test.depManager.Running(client)
	s().NoError(err)
	s().False(running)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDepManager(t *testing.T) {
	suite.Run(t, new(TestDepManagerSuite))
}

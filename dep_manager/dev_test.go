package dep_manager

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	cp "github.com/otiai10/copy"
	"github.com/pebbe/zmq4"
	clientConfig "github.com/sds-framework/client-lib/config"
	"github.com/sds-framework/log-lib"
	"github.com/sds-framework/os-lib/path"
	"github.com/stretchr/testify/suite"
)

// todo for public functions test with the nil values
// todo for public functions test with non linted dep

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

// Test_1_UrlToFileName tests the utility function that converts the URL into the file name.
func (test *TestDepManagerSuite) Test_1_UrlToFileName() {
	url := "github.com/sds-framework/test-ext"
	fileName := "github.com.ahmetson.test-ext"
	test.Require().Equal(urlToFileName(url), fileName)

	invalid := "github.com\\ahmetson\\test-ext"
	test.Require().Equal(urlToFileName(invalid), fileName)

	// with semicolon
	url = "::github.com/sds-framework/test-ext"
	test.Require().Equal(urlToFileName(url), fileName)

	// with space
	url = "::github.com/sds-framework/  test-ext  "
	test.Require().Equal(urlToFileName(url), fileName)
}

// todo change with testing Lint.
// Test_12_NewDep tests NewDep function and DepManager.Lint, Dep.IsLinted methods.
func (test *TestDepManagerSuite) Test_12_NewDep() {
	s := test.Require

	url := "github.com/sds-framework/test-manager"
	expectedSrcPath := filepath.Join(test.depManager.Src, "github.com.ahmetson.test-manager")
	expectedBinPath := path.BinPath(test.depManager.Bin, "github.com.ahmetson.test-manager")
	localSrcPath := path.AbsDir(test.currentDir, "_localSrc")

	// default dep managed by the DepManager
	dep, err := NewDep(url, "", "")
	s().NoError(err)

	s().False(dep.IsLinted())

	// trying to lint the nil values must not take any effect
	var depManager *DepManager
	depManager.Lint(dep)
	s().False(dep.IsLinted())
	// linting a nil value must not have any effect
	depManager.Lint(nil)

	// linted with the default values must return manageable dep
	test.depManager.Lint(dep)
	s().True(dep.IsLinted())
	s().True(dep.manageableSrc)
	s().True(dep.manageableBin)
	s().Len(dep.LocalUrl(), 0)
	s().Equal(expectedSrcPath, dep.srcPath)
	s().Equal(expectedBinPath, dep.binPath)

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
	test.depManager.Lint(dep)
	s().NotZero(len(dep.LocalUrl()))
	s().True(dep.IsLinted())
	s().False(dep.manageableSrc)
	s().True(dep.manageableBin)
	s().Equal(localSrcPath, dep.srcPath)

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
	test.depManager.Lint(dep)
	s().True(dep.IsLinted())
	s().False(dep.manageableBin)
	s().True(dep.manageableSrc)
	s().Zero(len(dep.LocalUrl()))
	s().Equal(localBin, dep.binPath)

	// clean out the parameters
	err = os.RemoveAll(localSrcPath)
	s().NoError(err)
}

// Test_13_downloadSrc makes sure to downloadSrc the remote repository into the context.
// This is the first part of Install.
// The second part of Install is building.
//
// Tests DepManager.downloadSrc and srcExist.
func (test *TestDepManagerSuite) Test_13_downloadSrc() {
	s := test.Require

	dep, err := NewDep(test.url, "", "")
	s().NoError(err)

	s().False(dep.IsLinted())
	test.depManager.Lint(dep)
	s().True(dep.IsLinted())

	// There should not be any source code before downloading
	exist, err := test.depManager.srcExist(dep)
	s().NoError(err)
	s().False(exist)

	// download the source code
	err = test.depManager.downloadSrc(dep, test.logger)
	s().NoError(err)

	// There should be a source code
	exist, _ = test.depManager.srcExist(dep)
	s().True(exist)

	// clean out the downloaded source code
	err = os.RemoveAll(dep.srcPath)
	s().NoError(err)
	exist, err = test.depManager.srcExist(dep)
	s().False(exist)
	s().NoError(err)

	//
	// Testing the failures
	//
	url := "github.com/sds-framework/no-repo" // this repo doesn't exist
	dep, err = NewDep(url, "", "")
	s().NoError(err)
	err = test.depManager.downloadSrc(dep, test.logger)
	s().Error(err)
}

// Test_14_build tests compiling a binary from the source code.
func (test *TestDepManagerSuite) Test_14_build() {
	s := test.Require

	localSrc := filepath.Join(test.localTestDir, "test-manager")

	dep, err := NewDep(test.url, localSrc, "")
	s().NoError(err)

	test.depManager.Lint(dep)
	s().True(dep.manageableBin)
	s().False(dep.manageableSrc)

	// There should not be any binary before building
	exist := test.depManager.Installed(dep)
	s().False(exist)

	// build the binaries
	err = test.depManager.build(dep, test.logger)
	s().NoError(err)

	// There should be a binary after testing
	exist = test.depManager.Installed(dep)
	s().True(exist)

	// remove the compiled binary
	err = os.Remove(dep.binPath)
	s().NoError(err)

	exist = test.depManager.Installed(dep)
	s().False(exist)
}

// Test_15_deleteSrc deletes the dependency's source code.
// The dependency was downloaded in Test_3_Download
func (test *TestDepManagerSuite) Test_15_deleteSrc() {
	s := test.Require

	dep, err := NewDep(test.url, "", "")
	s().NoError(err)
	test.depManager.Lint(dep)

	// There should not be a source code
	exist, err := test.depManager.srcExist(dep)
	s().False(exist)
	s().NoError(err)

	// copy the files to avoid downloading from the internet
	localSrcPath := filepath.Join(test.localTestDir, "test-manager")
	err = cp.Copy(localSrcPath, dep.srcPath)
	s().NoError(err)

	// make sure that files exist
	exist, err = test.depManager.srcExist(dep)
	s().NoError(err)
	s().True(exist)

	// Delete the source code
	err = test.depManager.deleteSrc(dep)
	s().NoError(err)

	// There should not be a source code
	exist, err = test.depManager.srcExist(dep)
	s().NoError(err)
	s().False(exist)
}

// Test_16_deleteBin deletes the dependency's binary.
// The binary was created by Test_4_Build
func (test *TestDepManagerSuite) Test_16_deleteBin() {
	s := test.Require

	dep, err := NewDep(test.url, "", "")
	s().NoError(err)
	test.depManager.Lint(dep)

	// The binary is not there as the tests are cleaned
	exist := test.depManager.Installed(dep)
	s().False(exist)

	localBinPath := path.BinPath(filepath.Join(test.localTestDir, "test-manager", "bin"), "test")
	err = cp.Copy(localBinPath, dep.binPath)
	s().NoError(err)

	// The binary must be presented
	// There should not be any binary before building
	exist = test.depManager.Installed(dep)
	s().True(exist)

	// no linted dep has can not return a binary
	dep, err = NewDep(test.url, "", "")
	s().NoError(err)
	exist = test.depManager.Installed(dep)
	s().False(exist)

	// but after lint it must return the files
	test.depManager.Lint(dep)
	exist = test.depManager.Installed(dep)
	s().True(exist)

	// Delete the binary
	err = test.depManager.deleteBin(dep)
	s().NoError(err)

	// The binary should be removed from the file
	exist = test.depManager.Installed(dep)
	s().False(exist)
}

// Test_17_Install is the combination of Test_3_Download and Test_4_Build.
// It's a functional test.
func (test *TestDepManagerSuite) Test_17_Install() {
	s := test.Require

	dep, err := NewDep(test.url, "", "")
	s().NoError(err)
	test.depManager.Lint(dep)

	// There should not be installed binary.
	// There should not be any binary before building
	exist := test.depManager.Installed(dep)
	s().False(exist)
	// no source code as well
	exist, err = test.depManager.srcExist(dep)
	s().NoError(err)
	s().False(exist)

	// installing dependencies must fail for nil values
	var depManager *DepManager
	err = depManager.Install(dep, test.logger)
	s().Error(err)
	err = test.depManager.Install(nil, test.logger)
	s().Error(err)

	// Install the dependency
	err = test.depManager.Install(dep, test.logger)
	s().NoError(err)

	// The binary should exist
	exist = test.depManager.Installed(dep)
	s().True(exist)

	// The source code must exist
	exist, err = test.depManager.srcExist(dep)
	s().NoError(err)
	s().True(exist)

	// trying to install when a binary exists must over-write the file and not download the source
	err = test.depManager.Install(dep, test.logger)
	s().NoError(err)
	exist = test.depManager.Installed(dep)
	s().True(exist)

	// even if the file is removed, binary must appear
	err = os.Remove(dep.binPath)
	s().NoError(err)

	exist = test.depManager.Installed(dep)
	s().False(exist)

	err = test.depManager.Install(dep, test.logger)
	s().NoError(err)
	exist = test.depManager.Installed(dep)
	s().True(exist)

	// not linted dep must fail
	dep, err = NewDep(test.url, "", "")
	s().NoError(err)
	err = test.depManager.Install(dep, test.logger)
	s().Error(err)
	test.depManager.Lint(dep)

	// non manageable dep must fail
	dep.manageableBin = false
	err = test.depManager.Install(dep, test.logger)
	s().Error(err)
	dep.manageableBin = true

	// source doesn't exist for non-manageable sources.
	// DepManager can't download non-manageable sources by design, therefore, it must fail
	origPath := dep.srcPath
	dep.srcPath = filepath.Join(test.currentDir, "_non_existing_directory")
	dep.manageableSrc = false
	err = test.depManager.Install(dep, test.logger)
	s().Error(err)

	dep.srcPath = origPath
	dep.manageableSrc = true
	err = test.depManager.Install(dep, test.logger)
	s().NoError(err)

	// clean out the binaries
	err = os.Remove(dep.binPath)
	s().NoError(err)
	err = os.RemoveAll(dep.srcPath)
	s().NoError(err)
	exist = test.depManager.Installed(dep)
	s().False(exist)
	exist, err = test.depManager.srcExist(dep)
	s().NoError(err)
	s().False(exist)

	// trying to install from the local binary must succeed
	localSrcPath := filepath.Join(test.localTestDir, "test-manager")
	dep, err = NewDep(test.url, localSrcPath, "")
	s().NoError(err)
	test.depManager.Lint(dep)
	exist, err = test.depManager.srcExist(dep)
	s().True(exist)
	s().False(dep.manageableSrc)
	exist = test.depManager.Installed(dep)
	s().False(exist)

	err = test.depManager.Install(dep, test.logger)
	s().NoError(err)
	exist = test.depManager.Installed(dep)
	s().True(exist)

	// clean out
	err = os.Remove(dep.binPath)
	s().NoError(err)
}

// Test_18_Uninstall is the combination of Test_5_DeleteSrc and Test_6_DeleteBin.
func (test *TestDepManagerSuite) Test_18_Uninstall() {
	s := test.Require

	localSrc := filepath.Join(test.localTestDir, "test-manager")
	localBin := path.BinPath(filepath.Join(localSrc, "bin"), "test")

	dep, err := NewDep(test.url, localSrc, localBin)
	s().NoError(err)
	test.depManager.Lint(dep)

	// non manageable dep has no action against the binary
	err = test.depManager.Uninstall(dep)
	s().Nil(err)

	// set the binaries
	dep, err = NewDep(test.url, localSrc, "")
	s().NoError(err)
	test.depManager.Lint(dep)

	err = cp.Copy(localBin, dep.binPath)
	s().NoError(err)

	// Test_7_Install should install the binary.
	exist := test.depManager.Installed(dep)
	s().True(exist)

	// Uninstall
	err = test.depManager.Uninstall(dep)
	s().NoError(err)

	// After uninstallation, we should not have the binary
	exist = test.depManager.Installed(dep)
	s().False(exist)

	// Uninstalling won't take any effect as the binary was already removed
	err = test.depManager.Uninstall(dep)
	s().NoError(err)

	// deleting the source code
	dep, err = NewDep(test.url, "", localBin)
	s().NoError(err)
	test.depManager.Lint(dep)

	// no source code before copying
	exist, err = test.depManager.srcExist(dep)
	s().NoError(err)
	s().False(exist)

	err = cp.Copy(localSrc, dep.srcPath)
	s().NoError(err)

	exist, err = test.depManager.srcExist(dep)
	s().NoError(err)
	s().True(exist)

	err = test.depManager.Uninstall(dep)
	s().NoError(err)

	exist, err = test.depManager.srcExist(dep)
	s().NoError(err)
	s().False(exist)

	// deleting source code that was deleted must not have any effect
	err = test.depManager.Uninstall(dep)
	s().NoError(err)
}

// Test_19_Uninstall is the combination of Test_5_DeleteSrc and Test_6_DeleteBin.
func (test *TestDepManagerSuite) Test_19_InvalidCompile() {
	s := test.Require

	localSrc := filepath.Join(test.localTestDir, "uncompilable")

	uncompilableDep, err := NewDep(test.url, localSrc, "")
	s().NoError(err)
	uncompilableDep.SetBranch("uncompilable")
	test.depManager.Lint(uncompilableDep)

	// Make sure that there is no binary before trying to install
	exist, err := test.depManager.srcExist(uncompilableDep)
	s().NoError(err)
	s().True(exist)
	exist = test.depManager.Installed(uncompilableDep)
	s().False(exist)

	// building must fail, since "uncompilable" branch code is not buildable
	err = test.depManager.build(uncompilableDep, test.logger)
	s().Error(err)
}

// Test_20_Run runs the given binary.
func (test *TestDepManagerSuite) Test_20_Run() {
	s := test.Require

	localBin := path.BinPath(filepath.Join(test.localTestDir, "test-manager", "bin"), "test")
	invalidBin := path.BinPath(filepath.Join(test.localTestDir, "test-manager", "bin"), "non_existing")

	dep, err := NewDep(test.url, "", localBin)
	s().NoError(err)
	test.depManager.Lint(dep)

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

	unLintedDep, err := NewDep(test.url, "", localBin)
	s().NoError(err)
	err = test.depManager.Run(unLintedDep, test.id, test.parent)
	s().Error(err) // dep is not linted

	// the binary doesn't exist
	invalidDep, err := NewDep(test.url, "", localBin)
	s().NoError(err)
	test.depManager.Lint(invalidDep)
	invalidDep.binPath = invalidBin
	err = test.depManager.Run(unLintedDep, test.id, test.parent)
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
	test.depManager.Lint(dep)

	// First, make sure that developer built the binary
	exist := test.depManager.Installed(dep)
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
	test.depManager.Lint(dep)

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

// Package dep_manager tracks the dependency manager in the local context.
package dep_manager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/sds-framework/client-lib"
	clientConfig "github.com/sds-framework/client-lib/config"
	"github.com/sds-framework/datatype-lib/data_type/key_value"
	"github.com/sds-framework/datatype-lib/message"
	"github.com/sds-framework/dev-lib/source"
	"github.com/sds-framework/log-lib"
	"github.com/sds-framework/os-lib/path"
)

// DefaultTimeout is the default time to wait before considering the message is not delivered.
// DepManager.Running method uses this value before considering the socket as not running.
const DefaultTimeout = time.Second

type Dep struct {
	*source.Src

	srcPath       string
	binPath       string
	manageableSrc bool
	manageableBin bool // if a binary was set by the user, then it's not updatable or deletable
	cmd           *exec.Cmd
	done          chan error // signalizes when the service finished
}

// A DepManager Manager builds, runs or stops the dependency services
type DepManager struct {
	runningDeps map[string]*Dep
	timeout     time.Duration

	Src string `json:"SERVICE_DEPS_SRC"` // Default Src path
	Bin string `json:"SERVICE_DEPS_BIN"`
}

// NewDep returns a dependency parameters. Pass empty strings if the dependency is managed by the DepManager.
func NewDep(url, localSrc, localBin string) (*Dep, error) {
	src, err := source.New(url, localSrc)
	if err != nil {
		return nil, fmt.Errorf("source.New('%s'): %w", url, err)
	}

	dep := &Dep{
		Src: src,
	}

	if len(localBin) > 0 {
		exist, err := path.FileExist(localBin)
		if !exist {
			if err != nil {
				err = fmt.Errorf("path.FileExist(localBin='%s'): %w", localBin, err)
			} else {
				err = fmt.Errorf("path.FileExist(localBin='%s'): false", localBin)
			}
			return nil, err
		}

		dep.binPath = localBin
	}

	return dep, nil
}

// New source manager in the Dev context.
//
// It will prepare the directories for source codes and binary.
// If preparation fails, it will throw an error.
func New() *DepManager {
	return &DepManager{
		Src:         "",
		Bin:         "",
		runningDeps: make(map[string]*Dep, 0),
		timeout:     DefaultTimeout,
	}
}

// IsLinted returns true if the Dep was linted with the DepManager.
func (dep *Dep) IsLinted() bool {
	// srcPath is set by Dep.Lint() method only.
	return len(dep.binPath) > 0 && len(dep.srcPath) > 0
}

func (dep *Dep) copy() *Dep {
	// no check against errors, as the Dep must have the valid source.
	src, _ := source.New(dep.Url, dep.LocalUrl())

	instance := &Dep{
		Src:           src,
		srcPath:       dep.srcPath,
		binPath:       dep.binPath,
		manageableBin: dep.manageableBin,
		manageableSrc: dep.manageableSrc,
		done:          make(chan error, 1),
	}

	return instance
}

// Lint sets the fields of Dep as for caching.
// The two primary flags are whether the Dep is managed by DepManager or not.
//
// The Dep source code is manageable if it doesn't have Dep.LocalUrl().
// The Dep binary is manageable if it binary path is not within the DepManager.Bin directory
func (manager *DepManager) Lint(dep *Dep) {
	if manager == nil || dep == nil {
		return
	}
	if dep.IsLinted() {
		return
	}

	// local bin was given
	if len(dep.binPath) > 0 {
		dir, _ := path.DirAndFileName(dep.binPath)
		i := strings.Index(dir, manager.Bin)
		dep.manageableBin = i == 0
	} else {
		dep.binPath = path.BinPath(manager.Bin, urlToFileName(dep.Url))
		dep.manageableBin = true
	}

	// local source code was given
	if len(dep.LocalUrl()) > 0 {
		dep.srcPath = dep.LocalUrl()

		dir, _ := path.DirAndFileName(dep.srcPath)
		i := strings.Index(dir, manager.Src)
		dep.manageableSrc = i == 0
	} else {
		dep.srcPath = filepath.Join(manager.Src, urlToFileName(dep.Url))
		dep.manageableSrc = true
	}
}

func (manager *DepManager) SetPaths(srcPath string, binPath string) error {
	if err := path.MakeDir(binPath); err != nil {
		return fmt.Errorf("path.MakeDir(%s): %w", binPath, err)
	}
	if err := path.MakeDir(srcPath); err != nil {
		return fmt.Errorf("path.MakeDir(%s): %w", srcPath, err)
	}

	manager.Src = srcPath
	manager.Bin = binPath

	return nil
}

// Close the dependency
func (manager *DepManager) Close(c *clientConfig.Client) error {
	// Make sure it's running
	running, err := manager.Running(c)
	if err != nil {
		return fmt.Errorf("manager.Running(client='%v'): %w", *c, err)
	}
	if !running {
		return nil
	}

	sock, err := client.New(c)
	if err != nil {
		return fmt.Errorf("zmq.NewSocket: %w", err)
	}

	closeRequest := &message.Request{
		Command:    "close",
		Parameters: key_value.New(),
	}

	sock.Timeout(manager.timeout).Attempt(1)

	_, err = sock.Request(closeRequest)
	if err == nil {
		return fmt.Errorf("socket.Request('close'): must exist with error since service closed before replying back")
	}

	running, err = manager.Running(c)
	if err != nil {
		return fmt.Errorf("socket.Request('close'): manager.Running(client='%v'): %w", *c, err)
	}

	if running {
		return fmt.Errorf("manager is running even after closing")
	}

	err = sock.Close()
	if err != nil {
		return fmt.Errorf("socket.Close: %w", err)
	}

	return nil
}

// Installed checks is the binary exist.
//
// Whether the depManager is manageable or not doesn't matter.
func (manager *DepManager) Installed(dep *Dep) bool {
	if manager == nil || dep == nil {
		return false
	}

	if !dep.IsLinted() {
		return false
	}

	exist, _ := path.FileExist(dep.binPath)
	return exist
}

// The srcExist checks is the source code exist or not.
// Since it is a private method, it assumes that depManager was linted.
func (manager *DepManager) srcExist(dep *Dep) (bool, error) {
	exists, err := path.DirExist(dep.srcPath)
	if err != nil {
		return false, fmt.Errorf("path.DirExists('%s'): %w", dep.srcPath, err)
	}
	return exists, nil
}

// Running checks whether the given client running or not.
// If the service is running on another process or on another node,
// then that service should expose the port.
func (manager *DepManager) Running(c *clientConfig.Client) (bool, error) {
	c.UrlFunc(clientConfig.Url)

	sock, err := client.New(c)
	if err != nil {
		return false, fmt.Errorf("client.New: %w", err)
	}
	sock.Attempt(1).Timeout(manager.timeout)

	req := &message.Request{
		Command:    "heartbeat",
		Parameters: key_value.New(),
	}

	_, err = sock.Request(req)
	if err != nil {
		return false, nil
	}

	closeErr := sock.Close()
	if closeErr != nil {
		return false, fmt.Errorf("socket.Close: %w", err)
	}

	return true, nil
}

// The build the application from source code.
// If the Dep is not manageable by DepManager, it returns an error.
//
// Since it's a private method, it assumes the depManager is linted, and its binary is manageable by DepManager.
func (manager *DepManager) build(dep *Dep, logger *log.Logger) error {
	err := cleanBuild(dep.srcPath, logger)
	if err != nil {
		return fmt.Errorf("cleanBuild(%s): %w", dep.srcPath, err)
	}

	cmd := exec.Command("go", "build", "-o", dep.binPath)
	cmd.Stdout = logger.Child("build", "binUrl", dep.binPath)
	cmd.Dir = dep.srcPath
	cmd.Stderr = logger.Child("buildErr", "binUrl", dep.binPath)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("cmd.Start: %w", err)
	}
	return nil
}

// OnStop returns a signal through the channel when the dependency spawned by the DepManager stops.
// If the dep is not existing, then it will simply return error.
func (manager *DepManager) OnStop(id string) chan error {
	dep, ok := manager.runningDeps[id]
	if !ok {
		return nil
	}

	if dep.cmd == nil {
		return nil
	}

	return dep.done
}

// Run runs the binary.
// If it fails to run, then it will return an error.
//
// Note that, services can crash during the initialization.
// In that case, you should use DepManager.OnStop method.
//
// If a parent is given, it's passed as ParentFlag.
// Todo, move all Flags from service-lib to config-lig.
// Todo, use the ParentFlag from the config lig
func (manager *DepManager) Run(dep *Dep, id string, optionalParent ...*clientConfig.Client) error {
	if manager == nil || dep == nil || len(id) == 0 {
		return fmt.Errorf("nil or no id")
	}
	if len(optionalParent) > 1 {
		return fmt.Errorf("too many optional parameters, either no parameter or 1 parameter required")
	}

	if !dep.IsLinted() {
		return fmt.Errorf("depManager is not linted. Call DepManager.Lint(Dep) first")
	}

	_, ok := manager.runningDeps[id]
	if ok {
		return fmt.Errorf("the dep with id '%s' already running", id)
	}

	ok = manager.Installed(dep)
	if !ok {
		return fmt.Errorf("no binary")
	}

	configFlag := fmt.Sprintf("--url=%s", dep.Url)
	idFlag := fmt.Sprintf("--id=%s", id)

	args := make([]string, 2, 3)
	args[0] = configFlag
	args[1] = idFlag

	if len(optionalParent) == 1 {
		parentKv, err := key_value.NewFromInterface(optionalParent[0])
		if err != nil {
			return fmt.Errorf("optionalParent: key_value.NewFromInterface(parent='%v'): %w", optionalParent[0], err)
		}
		parentFlag := fmt.Sprintf("--parent=%s", parentKv.String())
		args = append(args, parentFlag)
	}

	instance := dep.copy()

	manager.runningDeps[id] = instance

	logger, err := log.New(id, false)
	if err != nil {
		return fmt.Errorf("log.New('%s'): %w", id, err)
	}
	errLogger, err := log.New(id+"Err", false)
	if err != nil {
		return fmt.Errorf("log.New('%sErr'): %w", id, err)
	}

	cmd := exec.Command(dep.binPath, args...)
	cmd.Stdout = logger
	cmd.Stderr = errLogger
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("cmd.Start: %w", err)
	}

	instance.cmd = cmd
	manager.wait(id)

	return nil
}

// The wait is invoked if the spawned dependency stops.
// The dependencies are running asynchronously.
// In order to call this function, you must use the DepManager.Close() method.
// If the Close signal was sent to the spawned child, then
// this method will be called automatically by the operating system.
func (manager *DepManager) wait(id string) {
	go func() {
		err := manager.runningDeps[id].cmd.Wait() // it can return an error
		manager.runningDeps[id].done <- err
		delete(manager.runningDeps, id)
	}()
}

// downloadSrc gets the remote source code using Git.
//
// Since this is a private function, the callers must make sure that depManager is linted and no value is nil.
//
// The Dep may have a local src code.
// This method doesn't check for that.
// Therefore, if the Dep has a LocalUrl(), then don't call this method.
func (manager *DepManager) downloadSrc(dep *Dep, logger *log.Logger) error {
	if !dep.manageableSrc {
		return fmt.Errorf("source is not manageable by the DepManager")
	}

	options := &git.CloneOptions{
		URL:      dep.GitUrl,
		Progress: logger.Child("download"),
	}

	if len(dep.Branch) > 0 {
		options.ReferenceName = plumbing.NewBranchReferenceName(dep.Branch)
	}

	_, err := git.PlainClone(dep.srcPath, false, options)

	if err != nil {
		return fmt.Errorf("git.PlainClone --url %s --o %s: %w", dep.Url, dep.srcPath, err)
	}

	return nil
}

// The deleteSrc deletes the source code.
// Since this method is private, it assumes that depManager is linted and manageable.
func (manager *DepManager) deleteSrc(dep *Dep) error {
	err := os.RemoveAll(dep.srcPath)
	if err != nil {
		return fmt.Errorf("os.RemoveAll('%s'): %s", dep.srcPath, err)
	}

	return nil
}

// deleteBin deletes the binary from the directory.
// If there is no binary, it will throw an error.
// If attempt to delete failed, it will throw an error.
//
// This method is private, so it assumes Dep is linted by the caller.
func (manager *DepManager) deleteBin(dep *Dep) error {
	if !dep.manageableBin {
		return fmt.Errorf("depManager binary is not manageable by the DepManager")
	}

	if !manager.Installed(dep) {
		return fmt.Errorf("depManager '%s' not installed", dep.Url)
	}

	if err := os.Remove(dep.binPath); err != nil {
		return fmt.Errorf("os.Remove('%s'): %w", dep.binPath, err)
	}

	return nil
}

// Uninstall deletes the dependency source code, and its binary.
// Trying to uninstall already running application will fail.
//
// Uninstall will omit if no binary or source code exists.
// Uninstall won't take effect if depManager is not manageable.
func (manager *DepManager) Uninstall(dep *Dep) error {
	if manager == nil || dep == nil {
		return fmt.Errorf("nil")
	}

	if !dep.IsLinted() {
		return fmt.Errorf("depManager is not linted. Call DepManager.Lint(Dep) first")
	}

	if !dep.manageableBin && !dep.manageableSrc {
		return nil
	}

	if dep.manageableSrc {
		exist, err := manager.srcExist(dep)
		if err != nil {
			return fmt.Errorf("dep_manager.exist(%s): %w", dep.Url, err)
		}

		if exist {
			err := manager.deleteSrc(dep)
			if err != nil {
				return fmt.Errorf("source.deleteSrc: %w", err)
			}
		}
	}

	if dep.manageableBin {
		exist := manager.Installed(dep)
		if exist {
			err := manager.deleteBin(dep)
			if err != nil {
				return fmt.Errorf("source.deleteBin('%s'): %w", dep.Url, err)
			}
		}
	}

	return nil
}

// calls `go mod tidy`
func cleanBuild(srcUrl string, logger *log.Logger) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Stdout = logger.Child("clean")
	cmd.Dir = srcUrl
	cmd.Stderr = logger.Child("cleanErr")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("cmd.Start: %w", err)
	}

	return nil
}

// urlToFileName converts the given url to the file name. Simply it replaces the slashes with dots.
//
// Url returns the full url to connect to the orchestra.
//
// The orchestra url is defined from the main service's url.
//
// For example:
//
//	serviceUrl = "github.com/sds-framework/sample-service"
//	contextUrl = "orchestra.github.com.ahmetson.sample-service"
//
// This controllerName is set as the handler's name in the config.
// Then the handler package will generate an inproc:// url based on the handler name.
func urlToFileName(url string) string {
	str := strings.ReplaceAll(strings.ReplaceAll(url, "/", "."), "\\", ".")
	return regexp.MustCompile(`[^a-zA-Z0-9-_.]+`).ReplaceAllString(str, "")
}

// Package runtime contains the dependency runtime for the dev context.
package runtime

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/sds-framework/client-lib"
	clientConfig "github.com/sds-framework/client-lib/config"
	"github.com/sds-framework/datatype-lib/data_type/key_value"
	"github.com/sds-framework/datatype-lib/message"
	"github.com/sds-framework/dev-lib/source"
	"github.com/sds-framework/log-lib"
	"github.com/sds-framework/os-lib/path"
)

// DefaultTimeout is the default time to wait before considering the message is not delivered.
// Runtime.Running method uses this value before considering the socket as not running.
const DefaultTimeout = time.Second

type Dep struct {
	*source.Src

	binPath string
	cmd     *exec.Cmd
	done    chan error // signalizes when the service finished
}

// Runtime runs, stops, and checks dependency services.
type Runtime struct {
	runningDeps map[string]*Dep
	timeout     time.Duration

	Src string `json:"SERVICE_DEPS_SRC"` // Default Src path
	Bin string `json:"SERVICE_DEPS_BIN"`
}

// NewDep returns dependency parameters.
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

// New creates a dependency runtime in the Dev context.
//
// It will prepare the directories for source codes and binary.
// If preparation fails, it will throw an error.
func New() *Runtime {
	return &Runtime{
		Src:         "",
		Bin:         "",
		runningDeps: make(map[string]*Dep, 0),
		timeout:     DefaultTimeout,
	}
}

func (dep *Dep) copy() *Dep {
	// no check against errors, as the Dep must have the valid source.
	src, _ := source.New(dep.Url, dep.LocalUrl())

	instance := &Dep{
		Src:     src,
		binPath: dep.binPath,
		done:    make(chan error, 1),
	}

	return instance
}

func (rt *Runtime) SetPaths(srcPath string, binPath string) error {
	if err := path.MakeDir(binPath); err != nil {
		return fmt.Errorf("path.MakeDir(%s): %w", binPath, err)
	}
	if err := path.MakeDir(srcPath); err != nil {
		return fmt.Errorf("path.MakeDir(%s): %w", srcPath, err)
	}

	rt.Src = srcPath
	rt.Bin = binPath

	return nil
}

// Close the dependency
func (rt *Runtime) Close(c *clientConfig.Client) error {
	// Make sure it's running
	running, err := rt.Running(c)
	if err != nil {
		return fmt.Errorf("runtime.Running(client='%v'): %w", *c, err)
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

	sock.Timeout(rt.timeout).Attempt(1)

	_, err = sock.Request(closeRequest)
	if err == nil {
		return fmt.Errorf("socket.Request('close'): must exist with error since service closed before replying back")
	}

	running, err = rt.Running(c)
	if err != nil {
		return fmt.Errorf("socket.Request('close'): runtime.Running(client='%v'): %w", *c, err)
	}

	if running {
		return fmt.Errorf("runtime is running even after closing")
	}

	err = sock.Close()
	if err != nil {
		return fmt.Errorf("socket.Close: %w", err)
	}

	return nil
}

// binExist checks that the binary exists.
//
// Whether the runtime is manageable or not doesn't matter.
func (rt *Runtime) binExist(dep *Dep) bool {
	if rt == nil || dep == nil {
		return false
	}

	if len(dep.binPath) == 0 {
		return false
	}

	exist, _ := path.FileExist(dep.binPath)
	return exist
}

// Running checks whether the given client running or not.
// If the service is running on another process or on another node,
// then that service should expose the port.
func (rt *Runtime) Running(c *clientConfig.Client) (bool, error) {
	c.UrlFunc(clientConfig.Url)

	sock, err := client.New(c)
	if err != nil {
		return false, fmt.Errorf("client.New: %w", err)
	}
	sock.Attempt(1).Timeout(rt.timeout)

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

// OnStop returns a signal through the channel when the dependency spawned by the Runtime stops.
// If the dep is not existing, then it will simply return error.
func (rt *Runtime) OnStop(id string) chan error {
	dep, ok := rt.runningDeps[id]
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
// In that case, you should use Runtime.OnStop method.
//
// If a parent is given, it's passed as ParentFlag.
// Todo, move all Flags from service-lib to config-lig.
// Todo, use the ParentFlag from the config lig
func (rt *Runtime) Run(dep *Dep, id string, optionalParent ...*clientConfig.Client) error {
	if rt == nil || dep == nil || len(id) == 0 {
		return fmt.Errorf("nil or no id")
	}
	if len(optionalParent) > 1 {
		return fmt.Errorf("too many optional parameters, either no parameter or 1 parameter required")
	}

	if len(dep.binPath) == 0 {
		return fmt.Errorf("no binary")
	}

	_, ok := rt.runningDeps[id]
	if ok {
		return fmt.Errorf("the dep with id '%s' already running", id)
	}

	ok = rt.binExist(dep)
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

	rt.runningDeps[id] = instance

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
	rt.wait(id)

	return nil
}

// The wait is invoked if the spawned dependency stops.
// The dependencies are running asynchronously.
// In order to call this function, you must use the Runtime.Close() method.
// If the Close signal was sent to the spawned child, then
// this method will be called automatically by the operating system.
func (rt *Runtime) wait(id string) {
	go func() {
		err := rt.runningDeps[id].cmd.Wait() // it can return an error
		rt.runningDeps[id].done <- err
		delete(rt.runningDeps, id)
	}()
}

// Package topology manages dependency service lifecycle for noPerfection services.
package topology

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/client/sync_replier"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	"github.com/noPerfection/protocol/handler/control"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology/config"
)

// RuntimeInterface is implemented by the dependency runtime.
//
// It doesn't have the `Stop` command.
// Because, stopping must be done by the remote call from other services.
// Use it if you want to implement your own runtime.
type RuntimeInterface interface {
	// AddService registers a service in the runtime configuration.
	AddService(target config.DepTarget) error

	// SetService updates an existing service in the runtime configuration.
	SetService(service config.Service) error

	// RemoveService removes a service from the runtime configuration.
	RemoveService(serviceName string) error

	// StartService starts the dependency service with the given parent.
	StartService(serviceName string, optionalParent ...*ParentClient) (string, error)

	// IsServiceRunning checks is the service running or not.
	IsServiceRunning(serviceName string) (bool, error)

	// StopService stops the given dependency service.
	StopService(serviceName string) error
}

// DefaultTimeout is the default time to wait before considering the message is not delivered.
// Runtime.IsServiceRunning method uses this value before considering the endpoint as not running.
const DefaultTimeout = time.Second * 5

const ManagerHandlerCategory = "manager"

type Process struct {
	config *config.Service
	id     string
	cmd    *exec.Cmd
	done   chan error // signalizes when the service finished
}

// Runtime runs, stops, and checks dependency services.
type Runtime struct {
	config           *config.NoPerfection
	sameServices     map[string]int
	runningProcesses map[string]*Process
	timeout          time.Duration
}

var _ RuntimeInterface = (*Runtime)(nil)

// AddService registers a service target in the runtime configuration.
// A ref target must already resolve in the configuration, while an inline
// target and any inline dependencies are registered recursively.
func (rt *Runtime) AddService(target config.DepTarget) error {
	if rt == nil || rt.config == nil {
		return fmt.Errorf("nil config")
	}
	if err := config.ValidateDepTarget(target); err != nil {
		return fmt.Errorf("config.ValidateDepTarget: %w", err)
	}

	if target.Ref != "" {
		if err := rt.validateServiceRef(target.Ref, make(map[string]bool)); err != nil {
			return err
		}
		return nil
	}

	if err := rt.addInlineService(target.Inline, make(map[string]bool), rt.usedEndpoints()); err != nil {
		return err
	}

	return rt.config.Save()
}

func (rt *Runtime) validateServiceRef(serviceName string, visiting map[string]bool) error {
	service, err := rt.config.GetService(serviceName)
	if err != nil {
		return fmt.Errorf("rt.config.GetService('%s'): %w", serviceName, err)
	}
	if visiting[service.Name] {
		return fmt.Errorf("cycle detected at service '%s'", service.Name)
	}
	visiting[service.Name] = true
	defer delete(visiting, service.Name)
	if err := service.ValidateTypes(); err != nil {
		return fmt.Errorf("service.ValidateTypes('%s'): %w", service.Name, err)
	}

	for _, dep := range service.HandlerDeps {
		if err := rt.validateDepServiceTargets(dep, visiting); err != nil {
			return fmt.Errorf("service '%s' handler-deps category '%s': %w", service.Name, dep.Name, err)
		}
	}

	for _, handler := range service.Handlers {
		for _, dep := range handler.CommandDeps {
			if err := rt.validateDepServiceTargets(dep, visiting); err != nil {
				return fmt.Errorf("service '%s' command '%s': %w", service.Name, dep.Name, err)
			}
		}
	}

	return nil
}

func (rt *Runtime) validateDepServiceTargets(dep config.DepService, visiting map[string]bool) error {
	for _, target := range dep.Proxies {
		if err := rt.validateDepTargetExists(target, visiting); err != nil {
			return fmt.Errorf("proxy: %w", err)
		}
	}
	for _, target := range dep.Extensions {
		if err := rt.validateDepTargetExists(target, visiting); err != nil {
			return fmt.Errorf("extension: %w", err)
		}
	}
	return nil
}

func (rt *Runtime) validateDepTargetExists(target config.DepTarget, visiting map[string]bool) error {
	if err := config.ValidateDepTarget(target); err != nil {
		return err
	}
	if target.Ref != "" {
		return rt.validateServiceRef(target.Ref, visiting)
	}
	if _, err := rt.config.GetService(target.Inline.Name); err != nil {
		return fmt.Errorf("inline service '%s' is not registered: %w", target.Inline.Name, err)
	}
	return rt.validateServiceRef(target.Inline.Name, visiting)
}

func (rt *Runtime) addInlineService(service *config.Service, visiting map[string]bool, reservedEndpoints map[string]string) error {
	if service == nil {
		return fmt.Errorf("service is nil")
	}
	if len(service.Name) == 0 {
		return fmt.Errorf("service name is empty")
	}
	if visiting[service.Name] {
		return fmt.Errorf("cycle detected at service '%s'", service.Name)
	}
	visiting[service.Name] = true
	defer delete(visiting, service.Name)

	if err := service.ValidateTypes(); err != nil {
		return fmt.Errorf("service.ValidateTypes('%s'): %w", service.Name, err)
	}
	if service.Type == config.IndependentType {
		return fmt.Errorf("independent service can not be added")
	}
	if _, err := rt.config.GetService(service.Name); err == nil {
		return fmt.Errorf("service('%s') already added", service.Name)
	}
	if err := rt.reserveAvailableEndpoints(service, reservedEndpoints); err != nil {
		return err
	}

	for _, dep := range service.HandlerDeps {
		if err := rt.addOrValidateDepServiceTargets(dep, visiting, reservedEndpoints); err != nil {
			return fmt.Errorf("service '%s' handler-deps category '%s': %w", service.Name, dep.Name, err)
		}
	}

	for _, handler := range service.Handlers {
		for _, dep := range handler.CommandDeps {
			if err := rt.addOrValidateDepServiceTargets(dep, visiting, reservedEndpoints); err != nil {
				return fmt.Errorf("service '%s' command '%s': %w", service.Name, dep.Name, err)
			}
		}
	}

	if err := rt.config.SetService(*service); err != nil {
		return fmt.Errorf("rt.config.SetService: %w", err)
	}

	return nil
}

func (rt *Runtime) addOrValidateDepServiceTargets(dep config.DepService, visiting map[string]bool, reservedEndpoints map[string]string) error {
	for _, target := range dep.Proxies {
		if err := rt.addOrValidateNestedTarget(target, visiting, reservedEndpoints); err != nil {
			return fmt.Errorf("proxy: %w", err)
		}
	}
	for _, target := range dep.Extensions {
		if err := rt.addOrValidateNestedTarget(target, visiting, reservedEndpoints); err != nil {
			return fmt.Errorf("extension: %w", err)
		}
	}
	return nil
}

func (rt *Runtime) addOrValidateNestedTarget(target config.DepTarget, visiting map[string]bool, reservedEndpoints map[string]string) error {
	if err := config.ValidateDepTarget(target); err != nil {
		return err
	}
	if target.Ref != "" {
		return rt.validateServiceRef(target.Ref, visiting)
	}
	return rt.addInlineService(target.Inline, visiting, reservedEndpoints)
}

func (rt *Runtime) usedEndpoints() map[string]string {
	used := make(map[string]string)
	for _, service := range rt.config.Services {
		for _, handler := range service.Handlers {
			key, err := endpointKey(handler.Endpoint)
			if err != nil {
				continue
			}
			used[key] = fmt.Sprintf("service('%s') handler('%s')", service.Name, handler.Category)
		}
	}
	return used
}

func (rt *Runtime) reserveAvailableEndpoints(service *config.Service, reserved map[string]string) error {
	seen := make(map[string]struct{})
	for _, handler := range service.Handlers {
		key, err := endpointKey(handler.Endpoint)
		if err != nil {
			return fmt.Errorf("service('%s') handler('%s'): %w", service.Name, handler.Category, err)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("service('%s') has duplicate endpoint '%s'", service.Name, key)
		}
		seen[key] = struct{}{}

		if owner, exists := reserved[key]; exists {
			return fmt.Errorf("service('%s') handler('%s') endpoint '%s' is already used by %s", service.Name, handler.Category, key, owner)
		}
		reserved[key] = fmt.Sprintf("service('%s') handler('%s')", service.Name, handler.Category)
	}

	return nil
}

func endpointKey(endpoint message.Endpoint) (string, error) {
	if endpoint.Id == "" && !endpoint.IsRemote() {
		return "", fmt.Errorf("endpoint id is empty")
	}
	return fmt.Sprintf("%s:%d", endpoint.Id, endpoint.Port), nil
}

// SetService updates an existing service in the runtime configuration.
func (rt *Runtime) SetService(service config.Service) error {
	if rt == nil || rt.config == nil {
		return fmt.Errorf("nil config")
	}
	if len(service.Name) == 0 {
		return fmt.Errorf("service name is empty")
	}
	if err := config.ValidateServiceType(service.Type); err != nil {
		return fmt.Errorf("config.ValidateServiceType('%s'): %w", service.Type, err)
	}

	if service.Type == config.IndependentType {
		if err := rt.setIndependentService(service); err != nil {
			return err
		}

		return rt.config.Save()
	}

	if _, err := rt.config.GetService(service.Name); err != nil {
		return fmt.Errorf("rt.config.GetService('%s'): %w", service.Name, err)
	}

	if err := rt.config.SetService(service); err != nil {
		return fmt.Errorf("rt.config.SetService: %w", err)
	}

	return rt.config.Save()
}

func (rt *Runtime) setIndependentService(service config.Service) error {
	current, err := rt.config.GetByType(config.IndependentType)
	if err != nil {
		return fmt.Errorf("rt.config.GetByType('%s'): %w", config.IndependentType, err)
	}

	runtimeHandler, err := current.HandlerByCategory(RuntimeHandlerCategory)
	if err != nil {
		return fmt.Errorf("current.HandlerByCategory('%s'): %w", RuntimeHandlerCategory, err)
	}

	nextRuntimeHandler, err := service.HandlerByCategory(RuntimeHandlerCategory)
	if err != nil {
		nextRuntimeHandler = config.Handler{
			Type:     config.HandlerType(RuntimeSocketType),
			Category: RuntimeHandlerCategory,
		}
	}
	nextRuntimeHandler.Endpoint = runtimeHandler.Endpoint
	service.SetHandler(nextRuntimeHandler)

	if current.Name != service.Name {
		if err := rt.config.RemoveService(current.Name); err != nil {
			return fmt.Errorf("rt.config.RemoveService('%s'): %w", current.Name, err)
		}
	}

	if err := rt.config.SetService(service); err != nil {
		return fmt.Errorf("rt.config.SetService: %w", err)
	}

	return nil
}

// RemoveService removes a service from the runtime configuration.
func (rt *Runtime) RemoveService(serviceName string) error {
	if rt == nil || rt.config == nil {
		return fmt.Errorf("nil config")
	}
	if len(serviceName) == 0 {
		return fmt.Errorf("service name is empty")
	}

	if _, err := rt.config.GetService(serviceName); err != nil {
		return fmt.Errorf("rt.config.GetService('%s'): %w", serviceName, err)
	}

	running, err := rt.IsServiceRunning(serviceName)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("service('%s') is running, please stop it first", serviceName)
	}

	if err := rt.config.RemoveService(serviceName); err != nil {
		return err
	}

	if err := rt.config.Save(); err != nil {
		return fmt.Errorf("rt.config.Save: %w", err)
	}

	delete(rt.sameServices, serviceName)
	return nil
}

// New creates a dependency runtime in the Dev context.
func New(cfg *config.NoPerfection) *Runtime {
	return &Runtime{
		config:           cfg,
		sameServices:     make(map[string]int),
		runningProcesses: make(map[string]*Process, 0),
		timeout:          DefaultTimeout,
	}
}

func (process *Process) copy() *Process {
	return &Process{
		config: process.config,
		id:     process.id,
		done:   make(chan error, 1),
	}
}

// StopService stops the dependency service.
func (rt *Runtime) StopService(serviceName string) error {
	// Make sure it's running
	if rt == nil || rt.config == nil {
		return fmt.Errorf("nil config")
	}
	if len(serviceName) == 0 {
		return fmt.Errorf("service name is empty")
	}

	service, err := rt.config.GetService(serviceName)
	if err != nil {
		return err
	}
	if service.Type == config.IndependentType {
		return fmt.Errorf("service('%s') is independent service, impossible to stop since you are now using it", serviceName)
	}

	running, err := rt.IsServiceRunning(serviceName)
	if err != nil {
		return fmt.Errorf("runtime.IsServiceRunning('%s'): %w", serviceName, err)
	}
	if !running {
		return nil
	}

	managerControl, err := rt.managerControl(&service)
	if err != nil {
		return err
	}
	defer managerControl.Close()

	managerControl.Timeout(rt.timeout)
	managerControl.Attempt(1)
	if err := managerControl.HandlerClose(); err != nil {
		return fmt.Errorf("managerControl.HandlerClose: %w", err)
	}

	running, err = rt.IsServiceRunning(serviceName)
	if err != nil {
		return fmt.Errorf("managerControl.HandlerClose: runtime.IsServiceRunning('%s'): %w", serviceName, err)
	}

	if running {
		return fmt.Errorf("runtime is running even after closing")
	}

	return nil
}

// IsServiceRunning checks whether the given service is running or not.
func (rt *Runtime) IsServiceRunning(serviceName string) (bool, error) {
	if rt == nil || rt.config == nil {
		return false, fmt.Errorf("nil config")
	}
	if len(serviceName) == 0 {
		return false, fmt.Errorf("service name is empty")
	}

	service, err := rt.config.GetService(serviceName)
	if err != nil {
		return false, err
	}
	if service.Type == config.IndependentType {
		return true, nil
	}

	managerControl, err := rt.managerControl(&service)
	if err != nil {
		return false, err
	}
	defer managerControl.Close()

	managerControl.Attempt(1)
	managerControl.Timeout(rt.timeout)

	status, err := managerControl.HandlerStatus()
	if err != nil {
		return false, nil
	}

	return status == base.SocketReady, nil
}

// OnStop returns a signal through the channel when the process spawned by the Runtime stops.
// If the process is not existing, then it will simply return error.
func (rt *Runtime) OnStop(id string) chan error {
	process, ok := rt.runningProcesses[id]
	if !ok {
		return nil
	}

	if process.cmd == nil {
		return nil
	}

	return process.done
}

// GenerateId returns the next runtime id for a service name.
func (rt *Runtime) GenerateId(serviceName string) (string, error) {
	if rt == nil {
		return "", fmt.Errorf("nil runtime")
	}
	if len(serviceName) == 0 {
		return "", fmt.Errorf("service name is empty")
	}
	if rt.sameServices == nil {
		rt.sameServices = make(map[string]int)
	}

	count := rt.sameServices[serviceName]
	for {
		count++
		id := fmt.Sprintf("%s%d", serviceName, count)
		if _, exists := rt.runningProcesses[id]; !exists {
			rt.sameServices[serviceName]++
			return id, nil
		}
	}
}

func (rt *Runtime) refreshServiceCount(serviceName string) {
	count := 0
	for _, process := range rt.runningProcesses {
		if process != nil && process.config != nil && process.config.Name == serviceName {
			count++
		}
	}
	if count == 0 {
		delete(rt.sameServices, serviceName)
		return
	}
	rt.sameServices[serviceName] = count
}

func (rt *Runtime) managerControl(service *config.Service) (*sync_replier.BaseControl, error) {
	handler, err := service.HandlerByCategory(ManagerHandlerCategory)
	if err != nil {
		return nil, fmt.Errorf("no manager found in the '%s' service, please set its config", service.Name)
	}

	controlConfig := control.CreateInternalConfig(handlerConfig.New(
		handlerConfig.HandlerType(handler.Type),
		handler.Endpoint.Id,
		handler.Category,
		handler.Endpoint.Port,
	))

	managerControl, err := sync_replier.NewBaseControl(controlConfig.Id, controlConfig.Port)
	if err != nil {
		return nil, fmt.Errorf("sync_replier.NewBaseControl: %w", err)
	}

	return managerControl, nil
}

// StartService runs the service start command.
// If it fails to run, then it will return an error.
//
// Note that, services can crash during the initialization.
// In that case, you should use Runtime.OnStop method.
func (rt *Runtime) StartService(serviceName string, optionalParent ...*ParentClient) (string, error) {
	if rt == nil || rt.config == nil {
		return "", fmt.Errorf("nil config")
	}
	if len(serviceName) == 0 {
		return "", fmt.Errorf("service name is empty")
	}
	serviceConfig, err := rt.config.GetService(serviceName)
	if err != nil {
		return "", err
	}
	process := &Process{config: &serviceConfig}

	if len(optionalParent) > 1 {
		return "", fmt.Errorf("too many optional parameters, either no parameter or 1 parameter required")
	}
	if len(optionalParent) == 1 && optionalParent[0] == nil {
		return "", fmt.Errorf("nil parent")
	}

	if len(process.config.StartCommand) == 0 {
		return "", fmt.Errorf("no start command")
	}

	id, err := rt.GenerateId(process.config.Name)
	if err != nil {
		return "", fmt.Errorf("rt.GenerateId('%s'): %w", process.config.Name, err)
	}
	process.id = id

	idFlag := fmt.Sprintf("--id=%s", id)

	args := []string{idFlag}

	if len(optionalParent) == 1 {
		parentKv, err := datatype.NewFromInterface(optionalParent[0])
		if err != nil {
			rt.refreshServiceCount(process.config.Name)
			return "", fmt.Errorf("optionalParent: datatype.NewFromInterface(parent='%v'): %w", optionalParent[0], err)
		}
		parentFlag := fmt.Sprintf("--parent=%s", parentKv.String())
		args = append(args, parentFlag)
	}

	commandArgs := strings.Fields(process.config.StartCommand)
	if len(commandArgs) == 0 {
		rt.refreshServiceCount(process.config.Name)
		return "", fmt.Errorf("no start command")
	}

	instance := process.copy()

	rt.runningProcesses[id] = instance

	logger, err := log.New(id, false)
	if err != nil {
		delete(rt.runningProcesses, id)
		rt.refreshServiceCount(process.config.Name)
		return "", fmt.Errorf("log.New('%s'): %w", id, err)
	}
	errLogger, err := log.New(id+"Err", false)
	if err != nil {
		delete(rt.runningProcesses, id)
		rt.refreshServiceCount(process.config.Name)
		return "", fmt.Errorf("log.New('%sErr'): %w", id, err)
	}

	cmd := exec.Command(commandArgs[0], append(commandArgs[1:], args...)...)
	cmd.Stdout = logger
	cmd.Stderr = errLogger
	err = cmd.Start()
	if err != nil {
		delete(rt.runningProcesses, id)
		rt.refreshServiceCount(process.config.Name)
		return "", fmt.Errorf("cmd.Start: %w", err)
	}

	instance.cmd = cmd
	rt.wait(id)

	return id, nil
}

// The wait is invoked if the spawned dependency stops.
// The dependencies are running asynchronously.
// In order to call this function, you must use the Runtime.StopService() method.
// If the Close signal was sent to the spawned child, then
// this method will be called automatically by the operating system.
func (rt *Runtime) wait(id string) {
	go func() {
		process := rt.runningProcesses[id]
		err := process.cmd.Wait() // it can return an error
		process.done <- err
		delete(rt.runningProcesses, id)
		rt.refreshServiceCount(process.config.Name)
	}()
}

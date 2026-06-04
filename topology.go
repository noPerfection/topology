// Package topology manages dependency service lifecycle for noPerfection services.
package topology

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/noPerfection/log"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology/config"
)

type NodeInterface interface {
	// StopService stops the given dependency service.
	StopService(serviceName string) error

	// StartService starts the dependency service with the given parent name.
	StartService(serviceName string, optionalParent ...string) (string, error)

	// IsServiceRunning checks is the service running or not.
	IsServiceRunning(serviceName string) (bool, error)
}

// TopologyInterface is implemented by the dependency topology.
//
// It doesn't have the `Stop` command.
// Because, stopping must be done by the remote call from other services.
// Use it if you want to implement your own topology.
type TopologyInterface interface {
	NodeInterface

	// Service returns a service configuration by name.
	Service(serviceName string) (config.Service, error)

	// AddService registers a service in the topology configuration.
	AddService(target config.DepTarget) error

	// SetService updates an existing service in the topology configuration.
	SetService(service config.Service) error

	// RemoveService removes a service from the topology configuration.
	RemoveService(serviceName string) error
}

// DefaultTimeout is the default time to wait before considering the message is not delivered.
// Topology.IsServiceRunning method uses this value before considering the endpoint as not running.
const DefaultTimeout = time.Second * 5

const ServiceManagerCategory = "manager"

type Process struct {
	config *config.Service
	id     string
	cmd    *exec.Cmd
	done   chan error // signalizes when the service finished
}

// Topology runs, stops, and checks dependency services.
type Topology struct {
	config           *config.NoPerfection
	sameServices     map[string]int
	runningProcesses map[string]*Process
	timeout          time.Duration
}

var _ TopologyInterface = (*Topology)(nil)

// New creates a dependency topology in the Dev context.
func New(cfg *config.NoPerfection) *Topology {
	return &Topology{
		config:           cfg,
		sameServices:     make(map[string]int),
		runningProcesses: make(map[string]*Process, 0),
		timeout:          DefaultTimeout,
	}
}

// AddService registers a service target in the topology configuration.
// A ref target must already resolve in the configuration, while an inline
// target and any inline dependencies are registered recursively.
func (tp *Topology) AddService(target config.DepTarget) error {
	if tp == nil || tp.config == nil {
		return fmt.Errorf("nil config")
	}
	if err := config.ValidateDepTarget(target); err != nil {
		return fmt.Errorf("config.ValidateDepTarget: %w", err)
	}

	if target.Ref != "" {
		if err := tp.validateServiceRef(target.Ref, make(map[string]bool)); err != nil {
			return err
		}
		return nil
	}

	if err := tp.addInlineService(target.InlineService(), make(map[string]bool), tp.usedEndpoints()); err != nil {
		return err
	}

	return tp.config.Save()
}

// Service returns a service configuration by name.
func (tp *Topology) Service(serviceName string) (config.Service, error) {
	if tp == nil || tp.config == nil {
		return config.Service{}, fmt.Errorf("nil config")
	}

	service, err := tp.config.GetService(serviceName)
	if err != nil {
		return config.Service{}, fmt.Errorf("tp.config.GetService('%s'): %w", serviceName, err)
	}

	return service, nil
}

func (tp *Topology) validateServiceRef(serviceName string, visiting map[string]bool) error {
	service, err := tp.config.GetService(serviceName)
	if err != nil {
		return fmt.Errorf("tp.config.GetService('%s'): %w", serviceName, err)
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
		if err := tp.validateDepServiceTargets(dep, visiting); err != nil {
			return fmt.Errorf("service '%s' handler-deps category '%s': %w", service.Name, dep.Name, err)
		}
	}

	for _, handler := range service.Handlers {
		for _, dep := range handler.CommandDeps {
			if err := tp.validateDepServiceTargets(dep, visiting); err != nil {
				return fmt.Errorf("service '%s' command '%s': %w", service.Name, dep.Name, err)
			}
		}
	}

	return nil
}

func (tp *Topology) validateDepServiceTargets(dep config.DepService, visiting map[string]bool) error {
	for _, target := range dep.Proxies {
		if err := tp.validateDepTargetExists(target, visiting); err != nil {
			return fmt.Errorf("proxy: %w", err)
		}
	}
	for _, target := range dep.Extensions {
		if err := tp.validateDepTargetExists(target, visiting); err != nil {
			return fmt.Errorf("extension: %w", err)
		}
	}
	return nil
}

func (tp *Topology) validateDepTargetExists(target config.DepTarget, visiting map[string]bool) error {
	if err := config.ValidateDepTarget(target); err != nil {
		return err
	}
	if target.Ref != "" {
		return tp.validateServiceRef(target.Ref, visiting)
	}
	service := target.InlineService()
	if _, err := tp.config.GetService(service.Name); err != nil {
		return fmt.Errorf("inline service '%s' is not registered: %w", service.Name, err)
	}
	return tp.validateServiceRef(service.Name, visiting)
}

func (tp *Topology) addInlineService(service *config.Service, visiting map[string]bool, reservedEndpoints map[string]string) error {
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
	if _, err := tp.config.GetService(service.Name); err == nil {
		return fmt.Errorf("service('%s') already added", service.Name)
	}
	if err := tp.reserveAvailableEndpoints(service, reservedEndpoints); err != nil {
		return err
	}

	for _, dep := range service.HandlerDeps {
		if err := tp.addOrValidateDepServiceTargets(dep, visiting, reservedEndpoints); err != nil {
			return fmt.Errorf("service '%s' handler-deps category '%s': %w", service.Name, dep.Name, err)
		}
	}

	for _, handler := range service.Handlers {
		for _, dep := range handler.CommandDeps {
			if err := tp.addOrValidateDepServiceTargets(dep, visiting, reservedEndpoints); err != nil {
				return fmt.Errorf("service '%s' command '%s': %w", service.Name, dep.Name, err)
			}
		}
	}

	if err := tp.config.SetService(*service); err != nil {
		return fmt.Errorf("tp.config.SetService: %w", err)
	}

	return nil
}

func (tp *Topology) addOrValidateDepServiceTargets(dep config.DepService, visiting map[string]bool, reservedEndpoints map[string]string) error {
	for _, target := range dep.Proxies {
		if err := tp.addOrValidateNestedTarget(target, visiting, reservedEndpoints); err != nil {
			return fmt.Errorf("proxy: %w", err)
		}
	}
	for _, target := range dep.Extensions {
		if err := tp.addOrValidateNestedTarget(target, visiting, reservedEndpoints); err != nil {
			return fmt.Errorf("extension: %w", err)
		}
	}
	return nil
}

func (tp *Topology) addOrValidateNestedTarget(target config.DepTarget, visiting map[string]bool, reservedEndpoints map[string]string) error {
	if err := config.ValidateDepTarget(target); err != nil {
		return err
	}
	if target.Ref != "" {
		return tp.validateServiceRef(target.Ref, visiting)
	}
	return tp.addInlineService(target.InlineService(), visiting, reservedEndpoints)
}

func (tp *Topology) usedEndpoints() map[string]string {
	used := make(map[string]string)
	for _, service := range tp.config.Services {
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

func (tp *Topology) reserveAvailableEndpoints(service *config.Service, reserved map[string]string) error {
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

// SetService updates an existing service in the topology configuration.
func (tp *Topology) SetService(service config.Service) error {
	if tp == nil || tp.config == nil {
		return fmt.Errorf("nil config")
	}
	if len(service.Name) == 0 {
		return fmt.Errorf("service name is empty")
	}
	if err := config.ValidateServiceType(service.Type); err != nil {
		return fmt.Errorf("config.ValidateServiceType('%s'): %w", service.Type, err)
	}

	if service.Type == config.IndependentType {
		if err := tp.setIndependentService(service); err != nil {
			return err
		}

		return tp.config.Save()
	}

	if _, err := tp.config.GetService(service.Name); err != nil {
		return fmt.Errorf("tp.config.GetService('%s'): %w", service.Name, err)
	}

	if err := tp.config.SetService(service); err != nil {
		return fmt.Errorf("tp.config.SetService: %w", err)
	}

	return tp.config.Save()
}

func (tp *Topology) setIndependentService(service config.Service) error {
	current, err := tp.config.GetByType(config.IndependentType)
	if err != nil {
		if err := tp.config.SetService(service); err != nil {
			return fmt.Errorf("tp.config.SetService: %w", err)
		}
		return nil
	}

	topologyHandler, err := current.HandlerByCategory(TopologyHandlerCategory)
	if err == nil {
		nextTopologyHandler, err := service.HandlerByCategory(TopologyHandlerCategory)
		if err != nil {
			nextTopologyHandler = config.Handler{
				Type:     config.HandlerType(TopologySocketType),
				Category: TopologyHandlerCategory,
			}
		}
		nextTopologyHandler.Endpoint = topologyHandler.Endpoint
		service.SetHandler(nextTopologyHandler)
	}

	if current.Name != service.Name {
		if err := tp.config.RemoveService(current.Name); err != nil {
			return fmt.Errorf("tp.config.RemoveService('%s'): %w", current.Name, err)
		}
	}

	if err := tp.config.SetService(service); err != nil {
		return fmt.Errorf("tp.config.SetService: %w", err)
	}

	return nil
}

// RemoveService removes a service from the topology configuration.
func (tp *Topology) RemoveService(serviceName string) error {
	if tp == nil || tp.config == nil {
		return fmt.Errorf("nil config")
	}
	if len(serviceName) == 0 {
		return fmt.Errorf("service name is empty")
	}

	if _, err := tp.config.GetService(serviceName); err != nil {
		return fmt.Errorf("tp.config.GetService('%s'): %w", serviceName, err)
	}

	running, err := tp.IsServiceRunning(serviceName)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("service('%s') is running, please stop it first", serviceName)
	}

	if err := tp.config.RemoveService(serviceName); err != nil {
		return err
	}

	if err := tp.config.Save(); err != nil {
		return fmt.Errorf("tp.config.Save: %w", err)
	}

	delete(tp.sameServices, serviceName)
	return nil
}

//---------------------------------------------------------------------
//
// NodeInterface implementation
//
//---------------------------------------------------------------------

// StopService stops the dependency service.
func (tp *Topology) StopService(serviceName string) error {
	// Make sure it's running
	if tp == nil || tp.config == nil {
		return fmt.Errorf("nil config")
	}
	if len(serviceName) == 0 {
		return fmt.Errorf("service name is empty")
	}

	service, err := tp.config.GetService(serviceName)
	if err != nil {
		return err
	}
	if service.Type == config.IndependentType {
		return fmt.Errorf("service('%s') is independent service, impossible to stop since you are now using it", serviceName)
	}

	node, err := tp.newServiceManagerClient(&service)
	if err != nil {
		return err
	}
	defer node.Close()

	node.Timeout(tp.timeout)
	node.Attempt(2)

	running, err := node.IsServiceRunning(serviceName)
	if err != nil {
		return fmt.Errorf("node.IsServiceRunning('%s'): %w", serviceName, err)
	}
	if !running {
		return nil
	}

	if err := node.StopService(serviceName); err != nil {
		return fmt.Errorf("node.StopService('%s'): %w", serviceName, err)
	}

	running, err = node.IsServiceRunning(serviceName)
	if err != nil {
		return fmt.Errorf("StopService -> node.IsServiceRunning('%s'): %w", serviceName, err)
	}

	if running {
		return fmt.Errorf("topology is running even after closing")
	}

	return nil
}

// IsServiceRunning checks whether the given service is running or not.
func (tp *Topology) IsServiceRunning(serviceName string) (bool, error) {
	if tp == nil || tp.config == nil {
		return false, fmt.Errorf("nil config")
	}
	if len(serviceName) == 0 {
		return false, fmt.Errorf("service name is empty")
	}

	service, err := tp.config.GetService(serviceName)
	if err != nil {
		return false, err
	}
	if service.Type == config.IndependentType {
		return true, nil
	}

	node, err := tp.newServiceManagerClient(&service)
	if err != nil {
		return false, err
	}
	defer node.Close()

	node.Attempt(1)
	node.Timeout(tp.timeout)

	running, err := node.IsServiceRunning(serviceName)
	if err != nil {
		return false, nil
	}

	return running, nil
}

// OnStop returns a signal through the channel when the process spawned by the Topology stops.
// If the process is not existing, then it will simply return error.
func (tp *Topology) OnStop(id string) chan error {
	process, ok := tp.runningProcesses[id]
	if !ok {
		return nil
	}

	if process.cmd == nil {
		return nil
	}

	return process.done
}

// generateProcessId returns the next topology id for a service name.
func (tp *Topology) generateProcessId(serviceName string) (string, error) {
	if tp == nil {
		return "", fmt.Errorf("nil topology")
	}
	if len(serviceName) == 0 {
		return "", fmt.Errorf("service name is empty")
	}
	if tp.sameServices == nil {
		tp.sameServices = make(map[string]int)
	}

	count := tp.sameServices[serviceName]
	for {
		count++
		id := fmt.Sprintf("%s%d", serviceName, count)
		if _, exists := tp.runningProcesses[id]; !exists {
			tp.sameServices[serviceName]++
			return id, nil
		}
	}
}

func (tp *Topology) refreshServiceCount(serviceName string) {
	count := 0
	for _, process := range tp.runningProcesses {
		if process != nil && process.config != nil && process.config.Name == serviceName {
			count++
		}
	}
	if count == 0 {
		delete(tp.sameServices, serviceName)
		return
	}
	tp.sameServices[serviceName] = count
}

func (tp *Topology) newServiceManagerClient(service *config.Service) (*NodeClient, error) {
	handler, err := service.HandlerByCategory(ServiceManagerCategory)
	if err != nil {
		return nil, fmt.Errorf("no manager found in the '%s' service, please set its config", service.Name)
	}

	node, err := newNodeClient(handler.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("NewNode: %w", err)
	}

	return node, nil
}

// StartService runs the service start command.
// If it fails to run, then it will return an error.
//
// Note that, services can crash during the initialization.
// In that case, you should use Topology.OnStop method.
func (tp *Topology) StartService(serviceName string, optionalParent ...string) (string, error) {
	if tp == nil || tp.config == nil {
		return "", fmt.Errorf("nil config")
	}
	if len(serviceName) == 0 {
		return "", fmt.Errorf("service name is empty")
	}
	serviceConfig, err := tp.config.GetService(serviceName)
	if err != nil {
		return "", err
	}
	process := &Process{config: &serviceConfig}

	if len(optionalParent) > 1 {
		return "", fmt.Errorf("too many optional parameters, either no parameter or 1 parameter required")
	}
	if len(optionalParent) == 1 && optionalParent[0] == "" {
		return "", fmt.Errorf("empty parent")
	}

	node, err := tp.newServiceManagerClient(&serviceConfig)
	if err != nil {
		return "", err
	}
	defer node.Close()

	node.Attempt(1)
	node.Timeout(tp.timeout)

	running, err := node.IsServiceRunning(serviceName)
	if err != nil {
		return "", fmt.Errorf("StartService -> node.IsServiceRunning('%s'): %w", serviceName, err)
	}
	if running {
		return "", fmt.Errorf("service('%s') is already running", serviceName)
	}

	if len(process.config.StartCommand) == 0 {
		return "", fmt.Errorf("no start command")
	}

	id, err := tp.generateProcessId(process.config.Name)
	if err != nil {
		return "", fmt.Errorf("tp.generateProcessId('%s'): %w", process.config.Name, err)
	}
	process.id = id

	idFlag := fmt.Sprintf("--id=%s", id)

	args := []string{idFlag}

	if len(optionalParent) == 1 {
		parentFlag := fmt.Sprintf("--parent=%s", optionalParent[0])
		args = append(args, parentFlag)
	}

	commandArgs := strings.Fields(process.config.StartCommand)
	if len(commandArgs) == 0 {
		tp.refreshServiceCount(process.config.Name)
		return "", fmt.Errorf("no start command")
	}

	instance := process.copy()

	tp.runningProcesses[id] = instance

	logger, err := log.New(id, false)
	if err != nil {
		delete(tp.runningProcesses, id)
		tp.refreshServiceCount(process.config.Name)
		return "", fmt.Errorf("log.New('%s'): %w", id, err)
	}
	errLogger, err := log.New(id+"Err", false)
	if err != nil {
		delete(tp.runningProcesses, id)
		tp.refreshServiceCount(process.config.Name)
		return "", fmt.Errorf("log.New('%sErr'): %w", id, err)
	}

	cmd := exec.Command(commandArgs[0], append(commandArgs[1:], args...)...)
	cmd.Stdout = logger
	cmd.Stderr = errLogger
	err = cmd.Start()
	if err != nil {
		delete(tp.runningProcesses, id)
		tp.refreshServiceCount(process.config.Name)
		return "", fmt.Errorf("cmd.Start: %w", err)
	}

	instance.cmd = cmd
	tp.wait(id)

	return id, nil
}

// The wait is invoked if the spawned dependency stops.
// The dependencies are running asynchronously.
// In order to call this function, you must use the Topology.StopService() method.
// If the Close signal was sent to the spawned child, then
// this method will be called automatically by the operating system.
func (tp *Topology) wait(id string) {
	go func() {
		process := tp.runningProcesses[id]
		err := process.cmd.Wait() // it can return an error
		process.done <- err
		delete(tp.runningProcesses, id)
		tp.refreshServiceCount(process.config.Name)
	}()
}

func (process *Process) copy() *Process {
	return &Process{
		config: process.config,
		id:     process.id,
		done:   make(chan error, 1),
	}
}

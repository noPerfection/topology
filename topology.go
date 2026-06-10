// Package topology manages dependency service lifecycle for noPerfection services.
package topology

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/noPerfection/log"
	"github.com/noPerfection/topology/config"
)

type NodeInterface interface {
	// StopService stops the given dependency service.
	StopService(serviceName string) error

	// StartService starts the dependency service.
	StartService(serviceName string) (string, error)

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

	// Services returns the list of configured services.
	Services() ([]config.Service, error)

	// AddService registers a service in the topology configuration.
	AddService(record config.Service) error

	// SetService updates an existing service in the topology configuration.
	SetService(record config.Service) error

	// RemoveService removes a service from the topology configuration.
	RemoveService(serviceName string) error

	// StartServiceByConfig registers a service configuration and starts it.
	StartServiceByConfig(record config.Service) (string, error)

	// IsServiceRunningByManager checks whether a service manager is running.
	IsServiceRunningByManager(serviceName string, handler config.Handler) (bool, error)

	// StopServiceByManager stops the service behind the given service manager.
	StopServiceByManager(serviceName string, handler config.Handler) error
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

// AddService registers a service in the topology configuration.
func (tp *Topology) AddService(record config.Service) error {
	if tp == nil || tp.config == nil {
		return fmt.Errorf("nil config")
	}
	if record.IsZero() {
		return fmt.Errorf("service is empty")
	}
	if len(record.Name) == 0 {
		return fmt.Errorf("service name is empty")
	}
	if err := config.ValidateService(record); err != nil {
		return fmt.Errorf("config.ValidateService('%s'): %w", record.Name, err)
	}
	if _, err := tp.config.GetService(record.Name); err == nil {
		return fmt.Errorf("service('%s') already added", record.Name)
	}

	if err := tp.config.AddService(record); err != nil {
		return fmt.Errorf("tp.config.AddService: %w", err)
	}

	return tp.config.Save()
}

// Service returns a service configuration by name.
func (tp *Topology) Service(serviceName string) (config.Service, error) {
	if tp == nil || tp.config == nil {
		return config.Service{}, fmt.Errorf("nil config")
	}

	record, err := tp.config.GetService(serviceName)
	if err != nil {
		return config.Service{}, fmt.Errorf("tp.config.GetService('%s'): %w", serviceName, err)
	}

	return record, nil
}

// Services returns all configured services.
func (tp *Topology) Services() ([]config.Service, error) {
	if tp == nil || tp.config == nil {
		return nil, fmt.Errorf("nil config")
	}

	services := make([]config.Service, len(tp.config.Services))
	copy(services, tp.config.Services)

	return services, nil
}

// SetService updates an existing service in the topology configuration.
func (tp *Topology) SetService(record config.Service) error {
	if tp == nil || tp.config == nil {
		return fmt.Errorf("nil config")
	}
	if len(record.Name) == 0 {
		return fmt.Errorf("service name is empty")
	}
	if err := config.ValidateService(record); err != nil {
		return fmt.Errorf("config.ValidateService('%s'): %w", record.Name, err)
	}

	if err := tp.config.SetService(record); err != nil {
		return fmt.Errorf("tp.config.SetService: %w", err)
	}

	return tp.config.Save()
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

	process := tp.processForService(serviceName)
	if err := node.StopService(serviceName); err != nil {
		return fmt.Errorf("node.StopService('%s'): %w", serviceName, err)
	}

	if process == nil || process.done == nil {
		return nil
	}

	select {
	case <-process.done:
		return nil
	case <-time.After(tp.timeout * 3):
		return fmt.Errorf("service('%s') is still running after stop", serviceName)
	}
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

// IsServiceRunningByManager checks whether a service is running by directly
// contacting its manager handler.
func (tp *Topology) IsServiceRunningByManager(serviceName string, handler config.Handler) (bool, error) {
	if tp == nil {
		return false, fmt.Errorf("nil topology")
	}
	if len(serviceName) == 0 {
		return false, fmt.Errorf("service name is empty")
	}
	if handler.Category != ServiceManagerCategory {
		return false, fmt.Errorf("handler category must be %q", ServiceManagerCategory)
	}

	node, err := newNodeClient(handler.Endpoint)
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

// StopServiceByManager stops a service by directly contacting its manager
// handler.
func (tp *Topology) StopServiceByManager(serviceName string, handler config.Handler) error {
	if tp == nil {
		return fmt.Errorf("nil topology")
	}
	if len(serviceName) == 0 {
		return fmt.Errorf("service name is empty")
	}
	if handler.Category != ServiceManagerCategory {
		return fmt.Errorf("handler category must be %q", ServiceManagerCategory)
	}

	node, err := newNodeClient(handler.Endpoint)
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
		return fmt.Errorf("StopServiceByManager -> node.IsServiceRunning('%s'): %w", serviceName, err)
	}
	if running {
		return fmt.Errorf("topology is running even after closing")
	}

	return nil
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

func (tp *Topology) processForService(serviceName string) *Process {
	for _, process := range tp.runningProcesses {
		if process != nil && process.config != nil && process.config.Name == serviceName {
			return process
		}
	}
	return nil
}

func (tp *Topology) newServiceManagerClient(service *config.Service) (*NodeClient, error) {
	handler, err := service.HandlerByCategory(ServiceManagerCategory)
	if err != nil {
		return nil, fmt.Errorf("no manager found in the '%s' service, please set its config", service.Name)
	}

	node, err := newNodeClient(handler.AsHandler().Endpoint)
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
func (tp *Topology) StartService(serviceName string) (string, error) {
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

	node, err := tp.newServiceManagerClient(&serviceConfig)
	if err != nil {
		return "", err
	}
	defer node.Close()

	node.Attempt(1)
	node.Timeout(tp.timeout)

	running, err := node.IsServiceRunning(serviceName)
	if err == nil && running {
		return "", nil
	}

	return tp.startServiceConfig(serviceConfig)
}

func (tp *Topology) startServiceConfig(serviceConfig config.Service) (string, error) {
	process := &Process{config: &serviceConfig}

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

// StartServiceByConfig starts an IPC service from its full configuration without
// requiring it to be registered in the root topology.
func (tp *Topology) StartServiceByConfig(record config.Service) (string, error) {
	if tp == nil {
		return "", fmt.Errorf("nil topology")
	}
	if record.IsZero() {
		return "", fmt.Errorf("service is empty")
	}
	if len(record.Name) == 0 {
		return "", fmt.Errorf("service name is empty")
	}
	if err := config.ValidateService(record); err != nil {
		return "", fmt.Errorf("config.ValidateService('%s'): %w", record.Name, err)
	}
	if record.Type == config.IndependentType {
		return "", fmt.Errorf("independent service can not be started by config")
	}
	if !record.IsIpc() {
		return "", fmt.Errorf("service('%s') is not ipc service", record.Name)
	}
	if len(record.StartCommand) == 0 {
		return "", fmt.Errorf("service('%s') has no start command given", record.Name)
	}

	managerHandler, err := record.HandlerByCategory(ServiceManagerCategory)
	if err != nil {
		return "", fmt.Errorf("service %q manager handler: %w", record.Name, err)
	}
	running, err := tp.IsServiceRunningByManager(record.Name, managerHandler.AsHandler())
	if err != nil {
		return "", fmt.Errorf("tp.IsServiceRunningByManager('%s'): %w", record.Name, err)
	}
	if running {
		return "", nil
	}

	return tp.startServiceConfig(record)
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

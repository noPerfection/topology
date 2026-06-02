package topology

import (
	"fmt"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/message"
	config "github.com/noPerfection/topology/config"
)

type Client struct {
	socket *client.Socket
}

type ClientInterface interface {
	Close() error
	Timeout(duration time.Duration)
	Attempt(attempt uint8)

	StopService(serviceName string) error
	AddService(target config.DepTarget) error
	SetService(service config.Service) error
	RemoveService(serviceName string) error
	StartService(serviceName string, parent *ParentClient) (string, error)
	IsServiceRunning(serviceName string) (bool, error)
}

func NewClient(runtimeEndpoint message.Endpoint) (*Client, error) {
	socket, err := client.New(runtimeEndpoint.Id, runtimeEndpoint.Port, client.HandlerType(RuntimeSocketType))
	if err != nil {
		return nil, fmt.Errorf("client.New: %w", err)
	}

	return &Client{socket: socket}, nil
}

// Timeout of the client socket.
func (c *Client) Timeout(duration time.Duration) {
	c.socket.Timeout(duration)
}

// Attempt amount for requests.
func (c *Client) Attempt(attempt uint8) {
	c.socket.Attempt(attempt)
}

func (c *Client) Close() error {
	return c.socket.Close()
}

// StopService stops the running dependency service.
func (c *Client) StopService(serviceName string) error {
	req := message.Request{
		Command: StopService,
		Parameters: datatype.New().
			Set("service", serviceName),
	}

	if c == nil {
		return fmt.Errorf("dep manager not initialized")
	}

	if c.socket == nil {
		return fmt.Errorf("dep manager socket was closed")
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return fmt.Errorf("socket.Submit('%s'): %w", StopService, err)
	}

	if !reply.IsOK() {
		return fmt.Errorf("c.socket.Requeset(request='%v'): reply failed with: %s", req, reply.ErrorMessage())
	}

	return nil
}

// AddService registers a service target in the runtime configuration.
func (c *Client) AddService(target config.DepTarget) error {
	req := message.Request{
		Command: AddService,
		Parameters: datatype.New().
			Set("service", target),
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return fmt.Errorf("socket.Submit('%s'): %w", AddService, err)
	}

	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	return nil
}

// SetService updates an existing service in the runtime configuration.
func (c *Client) SetService(service config.Service) error {
	req := message.Request{
		Command: SetService,
		Parameters: datatype.New().
			Set("service", service),
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return fmt.Errorf("socket.Submit('%s'): %w", SetService, err)
	}

	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	return nil
}

// RemoveService removes a service from the runtime configuration.
func (c *Client) RemoveService(serviceName string) error {
	req := message.Request{
		Command: RemoveService,
		Parameters: datatype.New().
			Set("service", serviceName),
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return fmt.Errorf("socket.Submit('%s'): %w", RemoveService, err)
	}

	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	return nil
}

// StartService starts the dependency service and returns the generated runtime id.
func (c *Client) StartService(serviceName string, parent *ParentClient) (string, error) {
	req := message.Request{
		Command: StartService,
		Parameters: datatype.New().
			Set("parent", parent).
			Set("service", serviceName),
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return "", fmt.Errorf("socket.Submit('%s'): %w", StartService, err)
	}

	if !reply.IsOK() {
		return "", fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	id, err := reply.ReplyParameters().StringValue("id")
	if err != nil {
		return "", fmt.Errorf("reply.Parameters.GetString('id'): %w", err)
	}

	return id, nil
}

// IsServiceRunning checks is the service running or not.
func (c *Client) IsServiceRunning(serviceName string) (bool, error) {
	req := message.Request{
		Command: IsServiceRunning,
		Parameters: datatype.New().
			Set("service", serviceName),
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return false, fmt.Errorf("socket.Request('%s'): %w", IsServiceRunning, err)
	}

	if !reply.IsOK() {
		return false, fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	res, err := reply.ReplyParameters().BoolValue("running")
	if err != nil {
		return false, fmt.Errorf("reply.Parameters.GetBoolean('installed'): %w", err)
	}

	return res, nil
}

package topology

import (
	"fmt"
	"time"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/client"
	"github.com/noPerfection/protocol/message"
	"github.com/noPerfection/topology/config"
)

type Client struct {
	socket *client.Socket
}

// NodeClient is a topology protocol client connected to a service manager handler.
type NodeClient struct {
	*Client
}

var (
	_ NodeInterface     = (*NodeClient)(nil)
	_ TopologyInterface = (*Client)(nil)
)

// NewClient connects to the topology handler endpoint.
func NewClient() (*Client, error) {
	return newClient(message.NewEndpoint(TopologyHandlerCategory, 0))
}

// NewClient connects to the topology handler endpoint.
func newClient(serviceEndpoint message.Endpoint) (*Client, error) {
	socket, err := client.New(serviceEndpoint.Id, serviceEndpoint.Port, client.HandlerType(TopologySocketType))
	if err != nil {
		return nil, fmt.Errorf("client.New: %w", err)
	}

	return &Client{socket: socket}, nil
}

// newNodeClient connects to a service manager handler endpoint.
func newNodeClient(serviceEndpoint message.Endpoint) (*NodeClient, error) {
	c, err := newClient(serviceEndpoint)
	if err != nil {
		return nil, err
	}

	return &NodeClient{Client: c}, nil
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

// Service returns a service configuration by name.
func (c *Client) Service(serviceName string) (config.Service, error) {
	req := message.Request{
		Command: Service,
		Parameters: datatype.New().
			Set("service", serviceName),
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return config.Service{}, fmt.Errorf("socket.Request('%s'): %w", Service, err)
	}

	if !reply.IsOK() {
		return config.Service{}, fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	raw, err := reply.ReplyParameters().NestedValue("service")
	if err != nil {
		return config.Service{}, fmt.Errorf("reply.ReplyParameters().NestedValue('service'): %w", err)
	}

	var record config.Service
	if err := raw.Interface(&record); err != nil {
		return config.Service{}, fmt.Errorf("raw.Interface('config.Service'): %w", err)
	}

	return record, nil
}

// Services returns all service configurations.
func (c *Client) Services() ([]config.Service, error) {
	req := message.Request{
		Command:    Services,
		Parameters: datatype.New(),
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return nil, fmt.Errorf("socket.Request('%s'): %w", Services, err)
	}

	if !reply.IsOK() {
		return nil, fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	rawServices, err := reply.ReplyParameters().NestedListValue("services")
	if err != nil {
		return nil, fmt.Errorf("reply.ReplyParameters().NestedListValue('services'): %w", err)
	}

	records := make([]config.Service, 0, len(rawServices))
	for i, rawService := range rawServices {
		var record config.Service
		if err := rawService.Interface(&record); err != nil {
			return nil, fmt.Errorf("rawServices[%d].Interface('config.Service'): %w", i, err)
		}
		records = append(records, record)
	}

	return records, nil
}

// AddService registers a service in the topology configuration.
func (c *Client) AddService(record config.Service) error {
	req := message.Request{
		Command: AddService,
		Parameters: datatype.New().
			Set("service", record),
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

// SetService updates an existing service in the topology configuration.
func (c *Client) SetService(record config.Service) error {
	req := message.Request{
		Command: SetService,
		Parameters: datatype.New().
			Set("service", record),
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

// RemoveService removes a service from the topology configuration.
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

// StartService starts the dependency service and returns the generated topology id.
func (c *Client) StartService(serviceName string) (string, error) {
	parameters := datatype.New().Set("service", serviceName)

	req := message.Request{
		Command:    StartService,
		Parameters: parameters,
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

// StartServiceByConfig registers a service configuration, starts it, and returns
// the generated topology id.
func (c *Client) StartServiceByConfig(record config.Service) (string, error) {
	req := message.Request{
		Command: StartServiceByConfig,
		Parameters: datatype.New().
			Set("service", record),
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return "", fmt.Errorf("socket.Submit('%s'): %w", StartServiceByConfig, err)
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

// IsServiceRunningByManager checks whether a service is running by directly
// contacting its manager handler.
func (c *Client) IsServiceRunningByManager(serviceName string, handler config.Handler) (bool, error) {
	req := message.Request{
		Command: IsServiceRunningByManager,
		Parameters: datatype.New().
			Set("service", serviceName).
			Set("handler", handler),
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return false, fmt.Errorf("socket.Request('%s'): %w", IsServiceRunningByManager, err)
	}

	if !reply.IsOK() {
		return false, fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	res, err := reply.ReplyParameters().BoolValue("running")
	if err != nil {
		return false, fmt.Errorf("reply.Parameters.GetBoolean('running'): %w", err)
	}

	return res, nil
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

// StopServiceByManager stops the running dependency service by directly
// contacting its manager handler.
func (c *Client) StopServiceByManager(serviceName string, handler config.Handler) error {
	req := message.Request{
		Command: StopServiceByManager,
		Parameters: datatype.New().
			Set("service", serviceName).
			Set("handler", handler),
	}

	if c == nil {
		return fmt.Errorf("dep manager not initialized")
	}

	if c.socket == nil {
		return fmt.Errorf("dep manager socket was closed")
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return fmt.Errorf("socket.Submit('%s'): %w", StopServiceByManager, err)
	}

	if !reply.IsOK() {
		return fmt.Errorf("c.socket.Requeset(request='%v'): reply failed with: %s", req, reply.ErrorMessage())
	}

	return nil
}

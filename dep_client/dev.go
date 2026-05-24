package dep_client

import (
	"fmt"
	"time"

	"github.com/sds-framework/client-lib"
	clientConfig "github.com/sds-framework/client-lib/config"
	"github.com/sds-framework/datatype-lib/data_type/key_value"
	"github.com/sds-framework/datatype-lib/message"
	"github.com/sds-framework/dev-lib/dep_handler"
	handlerConfig "github.com/sds-framework/handler-lib/config"
)

type Client struct {
	socket *client.Socket
}

type Interface interface {
	Close() error
	Timeout(duration time.Duration)
	Attempt(attempt uint8)

	CloseDep(depClient *clientConfig.Client) error
	Uninstall(url string, localSrc string, localBin string) error
	Run(url string, id string, parent *clientConfig.Client, localBin string) error
	Install(url string, localSrc string) error
	Running(depClient *clientConfig.Client) (bool, error)
	Installed(url string, localBin string) (bool, error)
}

func New() (*Client, error) {
	configHandler := dep_handler.ServiceConfig()
	socketType := handlerConfig.SocketType(configHandler.Type)
	c := clientConfig.New("", configHandler.Id, configHandler.Port, socketType).
		UrlFunc(clientConfig.Url)

	socket, err := client.New(c)
	if err != nil {
		return nil, fmt.Errorf("client.New: %w", err)
	}

	return &Client{socket: socket}, nil
}

// Timeout of the client socket
func (c *Client) Timeout(duration time.Duration) {
	c.socket.Timeout(duration)
}

// Attempt amount for requests
func (c *Client) Attempt(attempt uint8) {
	c.socket.Attempt(attempt)
}

func (c *Client) Close() error {
	return c.socket.Close()
}

// CloseDep the running dependency
func (c *Client) CloseDep(depClient *clientConfig.Client) error {
	req := message.Request{
		Command: dep_handler.CloseDep,
		Parameters: key_value.New().
			Set("dep", depClient),
	}

	if c == nil {
		return fmt.Errorf("dep manager not initialized")
	}

	if c.socket == nil {
		return fmt.Errorf("dep manager socket was closed")
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return fmt.Errorf("socket.Submit('%s'): %w", dep_handler.CloseDep, err)
	}

	if !reply.IsOK() {
		return fmt.Errorf("c.socket.Requeset(request='%v'): reply failed with: %s", req, reply.ErrorMessage())
	}

	return nil
}

// Uninstall the dependency.
func (c *Client) Uninstall(url, localSrc, localBin string) error {
	req := message.Request{
		Command:    dep_handler.UninstallDep,
		Parameters: key_value.New().Set("url", url),
	}
	if len(localSrc) > 0 {
		req.Parameters.Set("local_src", localSrc)
	}
	if len(localBin) > 0 {
		req.Parameters.Set("local_bin", localBin)
	}

	err := c.socket.Submit(&req)
	if err != nil {
		return fmt.Errorf("socket.Submit('%s'): %w", dep_handler.UninstallDep, err)
	}

	return nil
}

// Run the dependency. The url of the dependency. It's id. and the parameters of the parent to connect to.
func (c *Client) Run(url string, id string, parent *clientConfig.Client, localBin string) error {
	req := message.Request{
		Command: dep_handler.RunDep,
		Parameters: key_value.New().
			Set("parent", parent).
			Set("url", url).
			Set("id", id),
	}
	if len(localBin) > 0 {
		req.Parameters.Set("local_bin", localBin)
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return fmt.Errorf("socket.Submit('%s'): %w", dep_handler.RunDep, err)
	}

	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	return nil
}

// Install the dependency from the source code. It compiles it.
func (c *Client) Install(url, localSrc string) error {
	req := message.Request{
		Command:    dep_handler.InstallDep,
		Parameters: key_value.New().Set("url", url),
	}
	if len(localSrc) > 0 {
		req.Parameters.Set("local_src", localSrc)
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return fmt.Errorf("socket.Submit('%s'): %w", dep_handler.InstallDep, err)
	}

	if !reply.IsOK() {
		return fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	return nil
}

// Running checks is the service running or not
func (c *Client) Running(depClient *clientConfig.Client) (bool, error) {
	req := message.Request{
		Command: dep_handler.DepRunning,
		Parameters: key_value.New().
			Set("dep", depClient),
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return false, fmt.Errorf("socket.Request('%s'): %w", dep_handler.DepRunning, err)
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

// Installed checks is the service installed
func (c *Client) Installed(url, localBin string) (bool, error) {
	req := message.Request{
		Command:    dep_handler.DepInstalled,
		Parameters: key_value.New().Set("url", url),
	}
	if len(localBin) > 0 {
		req.Parameters.Set("local_bin", localBin)
	}

	reply, err := c.socket.Request(&req)
	if err != nil {
		return false, fmt.Errorf("socket.Request('%s'): %w", dep_handler.DepInstalled, err)
	}

	if !reply.IsOK() {
		return false, fmt.Errorf("reply.Message: %s", reply.ErrorMessage())
	}

	res, err := reply.ReplyParameters().BoolValue("installed")
	if err != nil {
		return false, fmt.Errorf("reply.Parameters.GetBoolean('installed'): %w", err)
	}

	return res, nil
}

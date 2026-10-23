package connection

import (
	"fmt"

	tcclient "go.temporal.io/cloud-sdk/cloudclient"

	"github.com/temporalio/resource-operator/pkg/temporal/cloud/connection/loader"
)

// Connection wraps a Temporal client struct with an ability to load the client
// from various sources, including a Kubernetes Secret object or local files.
type Connection struct {
	tcc    *tcclient.Client
	loader *loader.Loader
}

// Client returns the Temporal Cloud client associated with the Connection.
func (c *Connection) Client() (*tcclient.Client, error) {
	if c.tcc != nil {
		return c.tcc, nil
	}
	if c.loader == nil {
		return nil, fmt.Errorf("no Loader configured for Connection")
	}

	clientOpts, err := c.loader.ClientOptions()
	if err != nil {
		return nil, err
	}
	client, err := tcclient.New(*clientOpts)
	if err != nil {
		return nil, err
	}
	c.tcc = client
	return client, nil
}

// Loader returns the Loader associated with the Connection, if any.
func (c *Connection) Loader() *loader.Loader {
	return c.loader
}

// NewOption allows modifying the returned Connection from New.
type NewOption func(*Connection)

// WithLoader modifies the Connection to use the supplied Loader.
func WithLoader(loader loader.Loader) NewOption {
	return func(c *Connection) {
		c.loader = &loader
	}
}

// WithClient overrides the Connection's Temporal Cloud Client with the
// supplied object.
func WithClient(tcc *tcclient.Client) NewOption {
	return func(c *Connection) {
		c.tcc = tcc
	}
}

// New returns a new Connection.
func New(opts ...NewOption) (*Connection, error) {
	c := Connection{}
	for _, opt := range opts {
		opt(&c)
	}

	if (c.tcc == nil && c.loader == nil) || (c.tcc != nil && c.loader != nil) {
		return nil, fmt.Errorf(
			"either use WithLoader or WithClient",
		)
	}

	return &c, nil
}

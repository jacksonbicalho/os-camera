package rtsp

import "errors"

type Connection interface {
	Open(url string) error
	IsOpen() bool
	ReadFrame() (Frame, error)
	Close() error
}

type Client struct {
	url       string
	conn      Connection
	connected bool
}

func NewClient(url string, conn Connection) *Client {
	return &Client{url: url, conn: conn}
}

func (c *Client) Connect() error {
	if err := c.conn.Open(c.url); err != nil {
		return err
	}
	c.connected = true
	return nil
}

func (c *Client) IsConnected() bool {
	return c.connected
}

func (c *Client) GetFrame() (Frame, error) {
	if !c.connected {
		return Frame{}, errors.New("client is not connected")
	}
	return c.conn.ReadFrame()
}

func (c *Client) Close() {
	c.conn.Close()
	c.connected = false
}

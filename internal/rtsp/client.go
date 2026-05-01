package rtsp

import (
	"errors"
	"log/slog"
)

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
	log       *slog.Logger
}

func NewClient(url string, conn Connection, log *slog.Logger) *Client {
	return &Client{url: url, conn: conn, log: log}
}

func (c *Client) Connect() error {
	c.log.Info("connecting", "url", c.url)
	if err := c.conn.Open(c.url); err != nil {
		c.log.Error("connection failed", "url", c.url, "error", err)
		return err
	}
	c.connected = true
	c.log.Info("connected", "url", c.url)
	return nil
}

func (c *Client) IsConnected() bool {
	return c.connected
}

func (c *Client) GetFrame() (Frame, error) {
	if !c.connected {
		return Frame{}, errors.New("client is not connected")
	}
	frame, err := c.conn.ReadFrame()
	if err != nil {
		c.log.Error("failed to read frame", "error", err)
		return Frame{}, err
	}
	c.log.Debug("frame received", "bytes", len(frame.Data))
	return frame, nil
}

func (c *Client) Close() {
	c.log.Info("closing connection", "url", c.url)
	c.conn.Close()
	c.connected = false
}

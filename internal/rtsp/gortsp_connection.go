package rtsp

import (
	"errors"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

type GortspConnection struct {
	client *gortsplib.Client
	frames chan Frame
	done   chan struct{}
	open   bool
}

func NewGortspConnection() *GortspConnection {
	return &GortspConnection{}
}

func (g *GortspConnection) Open(rawURL string) error {
	u, err := base.ParseURL(rawURL)
	if err != nil {
		return err
	}

	g.frames = make(chan Frame, 32)
	g.done = make(chan struct{})

	c := &gortsplib.Client{}
	c.OnPacketRTPAny(func(_ *description.Media, _ format.Format, pkt *rtp.Packet) {
		g.frames <- Frame{Data: pkt.Payload}
	})

	if err := c.Start(); err != nil {
		return err
	}

	desc, _, err := c.Describe(u)
	if err != nil {
		c.Close()
		return err
	}

	if err := c.SetupAll(desc.BaseURL, desc.Medias); err != nil {
		c.Close()
		return err
	}

	if _, err := c.Play(nil); err != nil {
		c.Close()
		return err
	}

	g.client = c
	g.open = true
	return nil
}

func (g *GortspConnection) IsOpen() bool {
	return g.open
}

func (g *GortspConnection) ReadFrame() (Frame, error) {
	if !g.open {
		return Frame{}, errors.New("connection is not open")
	}
	select {
	case frame := <-g.frames:
		return frame, nil
	case <-g.done:
		return Frame{}, errors.New("connection closed")
	}
}

func (g *GortspConnection) Close() error {
	if g.client != nil {
		g.client.Close()
	}
	if g.open {
		close(g.done)
	}
	g.open = false
	return nil
}

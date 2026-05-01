package rtsp

import "errors"

type FakeConnection struct {
	shouldOpen bool
	open       bool
	frameData  []byte
	readErr    error
}

func NewFakeConnection(shouldOpen bool, frameData []byte) *FakeConnection {
	return &FakeConnection{shouldOpen: shouldOpen, frameData: frameData}
}

func (f *FakeConnection) SetShouldOpen(v bool)      { f.shouldOpen = v }
func (f *FakeConnection) SetReadError(err error)    { f.readErr = err }

func (f *FakeConnection) Open(url string) error {
	if !f.shouldOpen {
		return errors.New("connection refused")
	}
	f.open = true
	return nil
}

func (f *FakeConnection) IsOpen() bool { return f.open }

func (f *FakeConnection) Close() error {
	f.open = false
	return nil
}

func (f *FakeConnection) ReadFrame() (Frame, error) {
	if f.readErr != nil {
		return Frame{}, f.readErr
	}
	if len(f.frameData) == 0 {
		return Frame{}, errors.New("no frame data")
	}
	return Frame{Data: f.frameData}, nil
}

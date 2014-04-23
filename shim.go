package gatt

import (
	"io"
	"os"
	"os/exec"
)

// A shim provides mediated access to BLE.
type shim interface {
	io.ReadWriteCloser
	Signal(os.Signal) error
	Wait() error
}

// cshim provides access to BLE via an external c executable.
type cshim struct {
	cmd *exec.Cmd
	io.Reader
	io.Writer
}

// newCShim starts the shim named file using the provided args.
func newCShim(file string, arg ...string) (shim, error) {
	c := new(cshim)
	var err error
	if file, err = exec.LookPath(file); err != nil {
		return nil, err
	}
	c.cmd = exec.Command(file, arg...)
	if c.Writer, err = c.cmd.StdinPipe(); err != nil {
		return nil, err
	}
	if c.Reader, err = c.cmd.StdoutPipe(); err != nil {
		return nil, err
	}
	if err = c.cmd.Start(); err != nil {
		return nil, err
	}
	return c, err
}

func (c *cshim) Wait() error                { return c.cmd.Wait() }
func (c *cshim) Close() error               { return c.cmd.Process.Kill() }
func (c *cshim) Signal(sig os.Signal) error { return c.cmd.Process.Signal(sig) }

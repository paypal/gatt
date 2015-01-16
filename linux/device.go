package linux

import (
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/paypal/gatt/linux/socket"
)

type device struct {
	fd  int
	rmu *sync.Mutex
	wmu *sync.Mutex
}

func newSocket(n int) (io.ReadWriteCloser, error) {
	fd, err := socket.Socket(socket.AF_BLUETOOTH, syscall.SOCK_RAW, socket.BTPROTO_HCI)

	// attempt to use the linux 3.14 feature, if this fails with EINVAL fall back to raw access
	// on older kernels
	sa := socket.SockaddrHCI{Dev: n, Channel: socket.HCI_CHANNEL_USER}
	if err = socket.Bind(fd, &sa); err == syscall.EINVAL {
		sa := socket.SockaddrHCI{Dev: n, Channel: socket.HCI_CHANNEL_RAW}
		if err = socket.Bind(fd, &sa); err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	return &device{
		fd:  fd,
		rmu: &sync.Mutex{},
		wmu: &sync.Mutex{},
	}, nil
}

func newDevice(path string) (io.ReadWriteCloser, error) {
	fd, err := syscall.Open(path, os.O_RDWR, 700)
	if err != nil {
		return nil, err
	}
	return &device{
		fd:  fd,
		rmu: &sync.Mutex{},
		wmu: &sync.Mutex{},
	}, nil
}

func (d device) Read(b []byte) (int, error) {
	d.rmu.Lock()
	defer d.rmu.Unlock()
	return syscall.Read(d.fd, b)
}

func (d device) Write(b []byte) (int, error) {
	d.wmu.Lock()
	defer d.wmu.Unlock()
	return syscall.Write(d.fd, b)
}

func (d device) Close() error {
	return syscall.Close(d.fd)
}

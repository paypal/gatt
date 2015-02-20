// Package socket implements a minimal set of function of the HCI Socket,
// which is not yet supported by the Go standard library. Most of the code
// follow suit the existing code in the standard library. Once it gets
// supported officially, we can get rid of this package entirely.

package socket

import (
	"log"
	"syscall"
	"time"
	"unsafe"
)

// Bluetooth Protocols
const (
	BTPROTO_L2CAP  = 0
	BTPROTO_HCI    = 1
	BTPROTO_SCO    = 2
	BTPROTO_RFCOMM = 3
	BTPROTO_BNEP   = 4
	BTPROTO_CMTP   = 5
	BTPROTO_HIDP   = 6
	BTPROTO_AVDTP  = 7
)

const (
	HCI_CHANNEL_RAW     = 0
	HCI_CHANNEL_USER    = 1
	HCI_CHANNEL_MONITOR = 2
	HCI_CHANNEL_CONTROL = 3
)

type _Socklen uint32

type Sockaddr interface {
	sockaddr() (ptr unsafe.Pointer, len _Socklen, err error) // lowercase; only we can define Sockaddrs
}

type rawSockaddrHCI struct {
	Family  uint16
	Dev     uint16
	Channel uint16
}

type SockaddrHCI struct {
	Dev     int
	Channel uint16
	raw     rawSockaddrHCI
}

const sizeofSockaddrHCI = unsafe.Sizeof(rawSockaddrHCI{})

func (sa *SockaddrHCI) sockaddr() (unsafe.Pointer, _Socklen, error) {
	if sa.Dev < 0 || sa.Dev > 0xFFFF {
		return nil, 0, syscall.EINVAL
	}
	if sa.Channel < 0 || sa.Channel > 0xFFFF {
		return nil, 0, syscall.EINVAL
	}
	sa.raw.Family = AF_BLUETOOTH
	sa.raw.Dev = uint16(sa.Dev)
	sa.raw.Channel = sa.Channel
	log.Println("sa:", sa)
	return unsafe.Pointer(&sa.raw), _Socklen(sizeofSockaddrHCI), nil
}

func Socket(domain, typ, proto int) (int, error) {
	for i := 0; i < 5; i++ {
		if fd, err := syscall.Socket(domain, typ, proto); err == nil || err != syscall.EBUSY {
			return fd, err
		}
		time.Sleep(time.Second)
	}
	return 0, syscall.EBUSY
}

func Bind(fd int, sa Sockaddr) (err error) {
	ptr, n, err := sa.sockaddr()
	if err != nil {
		log.Println("[BIND] first error:", err)
		return err
	}
	for i := 0; i < 5; i++ {
		if err = bind(fd, ptr, n); err == nil || err != syscall.EBUSY {
			log.Println("[BIND] second error:", err)
			return err
		}
		time.Sleep(time.Second)
	}
	log.Println("[BIND] busy")
	return syscall.EBUSY
}

// Socket Level
const (
	SOL_HCI    = 0
	SOL_L2CAP  = 6
	SOL_SCO    = 17
	SOL_RFCOMM = 18

	SOL_BLUETOOTH = 274
)

// HCI Socket options
const (
	HCI_DATA_DIR   = 1
	HCI_FILTER     = 2
	HCI_TIME_STAMP = 3
)

type HCIFilter struct {
	TypeMask  uint32
	EventMask [2]uint32
	Opcode    uint16
}

func SetsockoptFilter(fd int, f *HCIFilter) (err error) {
	return setsockopt(fd, SOL_HCI, HCI_FILTER, unsafe.Pointer(f), unsafe.Sizeof(*f))
}

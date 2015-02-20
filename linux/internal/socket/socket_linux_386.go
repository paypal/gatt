// +build linux,386

package socket

import (
	"log"
	"syscall"
	"unsafe"
)

const (
	BIND         = 2
	SETSOCKETOPT = 14
)

func bind(s int, addr unsafe.Pointer, addrlen _Socklen) (err error) {
	log.Println("addr:", addr)
	log.Println("addrLen:", addrlen)
	log.Println("s:", s)
	_, e1 := socketcall(BIND, uintptr(s), uintptr(addr), uintptr(addrlen), 0, 0, 0)
	if e1 != 0 {
		err = e1
	}
	log.Println("bind error:", err)
	return
}

func setsockopt(s int, level int, name int, val unsafe.Pointer, vallen uintptr) (err error) {
	_, e1 := socketcall(SETSOCKETOPT, uintptr(s), uintptr(level), uintptr(name), uintptr(val), vallen, 0)
	if e1 != 0 {
		err = e1
	}
	return
}

func socketcall(call int, a0, a1, a2, a3, a4, a5 uintptr) (n int, err syscall.Errno)

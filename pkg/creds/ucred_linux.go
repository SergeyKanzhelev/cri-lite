//go:build linux
// +build linux

package creds

import (
	"net"
	"syscall"
)

type ucred struct {
	pid int32
	uid uint32
	gid uint32
}

func getUcred(conn net.Conn) (*ucred, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, nil
	}

	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return nil, err
	}

	var cred *syscall.Ucred

	err = rawConn.Control(func(fd uintptr) {
		cred, err = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil {
		return nil, err
	}

	return &ucred{
		pid: cred.Pid,
		uid: cred.Uid,
		gid: cred.Gid,
	}, nil
}

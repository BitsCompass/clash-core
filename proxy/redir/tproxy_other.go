// +build !linux

package redir

import (
	"errors"
	"syscall"
)

func setsockopt(rc syscall.RawConn, addr string) error {
	return errors.New("not supported on current platform")
}

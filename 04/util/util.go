package util

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

const (
	KMaxMsg = 4096
)

func Msg(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

func Die(msg string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: [%d] %s\n", msg, err.(*os.SyscallError).Err, err)
	} else {
		fmt.Fprintln(os.Stderr, msg)
	}
	os.Exit(1)
}

func ReadFull(fd int, buf []byte, n int) error {
	for n > 0 {
		rv, err := syscall.Read(fd, buf)
		if rv == 0 {
			return io.EOF
		}
		if err != nil {
			return err
		}
		if rv > n {
			panic("readFull: rv is greater than n")
		}
		n -= rv
		buf = buf[rv:]
	}
	return nil
}

func WriteAll(fd int, buf []byte) error {
	n := len(buf)
	for n > 0 {
		rv, err := syscall.Write(fd, buf)
		if rv == 0 {
			return io.EOF
		}
		if err != nil {
			return err
		}
		if rv > n {
			panic("writeAll: rv is greater than n")
		}
		n -= rv
		buf = buf[rv:]
	}
	return nil
}

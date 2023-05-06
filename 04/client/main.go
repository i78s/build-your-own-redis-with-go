package main

import (
	"byor/04/util"
	"fmt"
	"syscall"
	"unsafe"
)

func query(fd int, text string) error {
	length := uint32(len(text))
	if length > util.KMaxMsg {
		return fmt.Errorf("too long")
	}

	wbuf := make([]byte, 4+length)
	*(*uint32)(unsafe.Pointer(&wbuf[0])) = length
	copy(wbuf[4:], text)

	err := util.WriteAll(fd, wbuf)
	if err != nil {
		return err
	}

	rbuf := make([]byte, 4+util.KMaxMsg+1)
	err = util.ReadFull(fd, rbuf[:4], 4)
	if err != nil {
		return err
	}

	length = *(*uint32)(unsafe.Pointer(&rbuf[0]))
	if length > util.KMaxMsg {
		return fmt.Errorf("too long")
	}

	err = util.ReadFull(fd, rbuf[4:length+4], int(length))
	if err != nil {
		return err
	}

	fmt.Printf("server says: %s\n", string(rbuf[4:4+length]))
	return nil
}

func main() {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		panic(err)
	}

	addr := syscall.SockaddrInet4{
		Port: 1234,
		Addr: [4]byte{127, 0, 0, 1},
	}

	err = syscall.Connect(fd, &addr)
	if err != nil {
		panic(err)
	}

	err = query(fd, "hello1")
	if err != nil {
		goto L_DONE
	}
	err = query(fd, "hello2")
	if err != nil {
		goto L_DONE
	}
	err = query(fd, "hello3")
	if err != nil {
		goto L_DONE
	}

L_DONE:
	syscall.Close(fd)
	// os.Exit(0)
}

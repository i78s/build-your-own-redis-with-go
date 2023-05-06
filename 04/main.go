package main

import (
	"byor/04/util"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"syscall"
)

func oneRequest(connfd int) error {
	header := make([]byte, 4)
	err := util.ReadFull(connfd, header, 4)
	if err != nil {
		if err == io.EOF {
			fmt.Println("EOF")
		} else {
			fmt.Println("read() error", err)
		}
		return err
	}
	length := binary.LittleEndian.Uint32(header)
	if length > util.KMaxMsg {
		return fmt.Errorf("too long")
	}
	body := make([]byte, length)
	err = util.ReadFull(connfd, body, int(length))
	if err != nil {
		return err
	}
	fmt.Printf("client says: %s\n", string(body))

	reply := "world"
	header = make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(reply)))
	payload := append(header, []byte(reply)...)
	return util.WriteAll(connfd, payload)
}

func main() {
	// Create socket
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		util.Die("socket()", err)
	}

	// Set SO_REUSEADDR
	err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if err != nil {
		util.Die("setsockopt()", err)
	}

	// Bind
	addr := &syscall.SockaddrInet4{Port: 1234}
	copy(addr.Addr[:], net.IPv4(0, 0, 0, 0)) // Wildcard address 0.0.0.0
	err = syscall.Bind(fd, addr)
	if err != nil {
		util.Die("bind()", err)
	}

	// Listen
	err = syscall.Listen(fd, syscall.SOMAXCONN)
	if err != nil {
		util.Die("listen()", err)
	}

	for {
		connfd, _, err := syscall.Accept(fd)

		if err != nil {
			continue
		}

		// only serves one client connection at once
		for {
			err := oneRequest(connfd)
			if err != nil {
				break
			}
		}
		syscall.Close(connfd)
	}
}

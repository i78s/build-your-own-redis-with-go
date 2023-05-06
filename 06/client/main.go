package main

import (
	"byor/04/util"
	"encoding/binary"
	"fmt"
	"syscall"
)

func sendReq(fd int, text string) error {
	length := uint32(len(text))
	if length > util.KMaxMsg {
		return fmt.Errorf("too long")
	}

	wbuf := make([]byte, 4+util.KMaxMsg)
	binary.BigEndian.PutUint32(wbuf[:4], length)
	copy(wbuf[4:], []byte(text))
	err := util.WriteAll(fd, wbuf[:4+length])

	return err
}

func readRes(fd int) error {
	rbuf := make([]byte, 4+util.KMaxMsg+1)
	err := util.ReadFull(fd, rbuf[:4], 4)
	if err != nil {
		return err
	}

	length := binary.BigEndian.Uint32(rbuf[:4])
	if length > util.KMaxMsg {
		return fmt.Errorf("too long")
	}

	// reply body
	err = util.ReadFull(fd, rbuf[4:length+4], int(length))
	if err != nil {
		return err
	}

	// do something
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

	// Multiple pipelined requests
	queryList := []string{
		"hello1",
		"hello2",
		"hello3",
	}
	for _, query := range queryList {
		fmt.Println("bf send", query)
		err := sendReq(fd, query)
		fmt.Println("af send")
		if err != nil {
			fmt.Println("sendReq() error:", err)
			goto L_DONE
		}
	}

	for range queryList {
		err := readRes(fd)
		if err != nil {
			fmt.Println("readRes() error:", err)
			goto L_DONE
		}
	}

L_DONE:
	syscall.Close(fd)
	// os.Exit(0)
}

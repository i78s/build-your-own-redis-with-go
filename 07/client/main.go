package main

import (
	"byor/04/util"
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
)

func sendReq(fd int, cmd []string) error {
	length := uint32(4)
	for _, s := range cmd {
		length += uint32(4 + len(s))
	}
	if length > util.KMaxMsg {
		return fmt.Errorf("too long")
	}

	wbuf := make([]byte, 4+util.KMaxMsg)
	binary.LittleEndian.PutUint32(wbuf[:4], uint32(length))
	n := uint32(len(cmd))
	binary.LittleEndian.PutUint32(wbuf[4:8], n)
	cur := 8
	for _, s := range cmd {
		p := uint32(len(s))
		binary.LittleEndian.PutUint32(wbuf[cur:cur+4], p)
		copy(wbuf[cur+4:cur+4+len(s)], s)
		cur += 4 + len(s)
	}

	return util.WriteAll(fd, wbuf[:4+length])
}

func readRes(fd int) error {
	rbuf := make([]byte, 4+util.KMaxMsg+1)
	err := util.ReadFull(fd, rbuf[:4], 4)
	if err != nil {
		return err
	}

	length := binary.LittleEndian.Uint32(rbuf[:4])
	if length > util.KMaxMsg {
		return fmt.Errorf("too long")
	}

	// reply body
	err = util.ReadFull(fd, rbuf[4:length+4], int(length))
	if err != nil {
		return err
	}

	// print the result
	resCode := binary.LittleEndian.Uint32(rbuf[4:8])
	fmt.Printf("server says: [%d] %s\n", resCode, string(rbuf[8:8+length-4]))

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

	cmd := os.Args[1:]

	err = sendReq(fd, cmd)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		goto L_DONE
	}

	err = readRes(fd)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		goto L_DONE
	}

L_DONE:
	syscall.Close(fd)
	// os.Exit(0)
}

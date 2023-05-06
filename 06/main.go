package main

import (
	"byor/04/util"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func fdSetNonBlocking(fd int) error {
	flags, _, err := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(syscall.F_GETFL), 0)
	if err != 0 {
		return os.NewSyscallError("fcntl", err)
	}

	flags |= syscall.O_NONBLOCK

	_, _, err = syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(syscall.F_SETFL), uintptr(flags))
	if err != 0 {
		return os.NewSyscallError("fcntl", err)
	}

	return nil
}

type ConnectionState int

const (
	StateReq ConnectionState = iota
	StateRes
	StateEnd
)

type Conn struct {
	fd       int
	state    ConnectionState
	rbufSize int
	rbuf     [4 + util.KMaxMsg]byte
	wbufSize int
	wbufSent int
	wbuf     [4 + util.KMaxMsg]byte
}

func connPut(fd2conn *[]*Conn, conn *Conn) {
	if len(*fd2conn) <= conn.fd {
		*fd2conn = append(*fd2conn, make([]*Conn, conn.fd-len(*fd2conn)+1)...)
	}
	(*fd2conn)[conn.fd] = conn
}

func acceptNewConn(fd2conn *[]*Conn, fd int) int32 {
	// Accept
	connfd, _, err := syscall.Accept(fd)
	if err != nil {
		util.Msg("accept() error")
		return -1 // error
	}

	// Set the new connection fd to nonblocking mode
	fdSetNonBlocking(connfd)
	// Create the Conn struct
	conn := &Conn{
		fd:       connfd,
		state:    StateReq,
		rbufSize: 0,
		wbufSize: 0,
		wbufSent: 0,
	}
	connPut(fd2conn, conn)
	return 0
}

func tryOneRequest(conn *Conn) bool {
	// Try to parse a request from the buffer
	if conn.rbufSize < 4 {
		// Not enough data in the buffer. Will retry in the next iteration
		return false
	}
	len := binary.BigEndian.Uint32(conn.rbuf[:4])
	if len > util.KMaxMsg {
		util.Msg("too long 1")
		conn.state = StateEnd
		return false
	}
	if 4+int(len) > conn.rbufSize {
		// Not enough data in the buffer. Will retry in the next iteration
		return false
	}

	// Got one request, do something with it
	fmt.Printf("client says: %.*s\n", len, conn.rbuf[4:4+len])

	// Generating echoing response
	binary.BigEndian.PutUint32(conn.wbuf[:4], len)
	copy(conn.wbuf[4:], conn.rbuf[4:4+len])
	conn.wbufSize = int(4 + len)

	// Remove the request from the buffer
	// Note: Frequent copy is inefficient
	// Note: Need better handling for production code
	remain := conn.rbufSize - (4 + int(len))
	if remain > 0 {
		copy(conn.rbuf[:], conn.rbuf[4+len:int(4+len)+remain])
	}
	conn.rbufSize = remain

	// Change state
	conn.state = StateRes
	stateRes(conn)

	// Continue the outer loop if the request was fully processed
	return conn.state == StateReq
}

func tryFillBuffer(conn *Conn) bool {
	// Try to fill the buffer
	if conn.rbufSize >= len(conn.rbuf) {
		panic("Buffer size exceeded")
	}
	rv := int64(0)
	for {
		capacity := len(conn.rbuf) - conn.rbufSize
		n, err := syscall.Read(conn.fd, conn.rbuf[conn.rbufSize:conn.rbufSize+capacity])
		rv = int64(n)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			if err == syscall.EAGAIN {
				// got EAGAIN, stop.
				return false
			}
			util.Msg("read() error")
			conn.state = StateEnd
			return false
		}
		break
	}

	if rv == 0 {
		if conn.rbufSize > 0 {
			util.Msg("unexpected EOF")
		} else {
			util.Msg("EOF")
		}
		conn.state = StateEnd
		return false
	}

	conn.rbufSize += int(rv)

	if conn.rbufSize > len(conn.rbuf)-conn.rbufSize {
		panic("Buffer size exceeded")
	}

	// Try to process requests one by one.
	// Why is there a loop? Please read the explanation of "pipelining".
	for tryOneRequest(conn) {
	}

	return conn.state == StateReq
}

func stateReq(conn *Conn) {
	for tryFillBuffer(conn) {
	}
}

func tryFlushBuffer(conn *Conn) bool {
	var rv int
	for {
		remain := conn.wbufSize - conn.wbufSent
		_rv, err := syscall.Write(conn.fd, conn.wbuf[conn.wbufSent:conn.wbufSent+remain])
		rv = _rv
		if err == nil {
			break
		}
		if err == syscall.EINTR {
			continue
		}
		if err == syscall.EAGAIN {
			return false
		}
		util.Msg("write() error")
		conn.state = StateEnd
		return false
	}
	conn.wbufSent += rv
	if conn.wbufSent > conn.wbufSize {
		panic("conn.wbufSent > conn.wbufSize")
	}
	if conn.wbufSent == conn.wbufSize {
		conn.state = StateReq
		conn.wbufSent = 0
		conn.wbufSize = 0
		return false
	}
	return true
}

func stateRes(conn *Conn) {
	for tryFlushBuffer(conn) {
	}
}

func connectionIO(conn *Conn) {
	if conn.state == StateReq {
		fmt.Println("connectionIO StateReq")
		stateReq(conn)
	} else if conn.state == StateRes {
		fmt.Println("connectionIO StateRes")
		stateRes(conn)
	} else {
		panic("unexpected state") // Not expected
	}
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

	// A slice of all client connections
	fd2conn := make([]*Conn, 0)
	// Set the listen fd to nonblocking mode
	fdSetNonBlocking(fd)
	// The event loop
	pollArgs := make([]unix.PollFd, 0)

	for {
		// Prepare the arguments of the poll()
		pollArgs = pollArgs[:0]
		// For convenience, the listening fd is put in the first position
		pollArgs = append(pollArgs, unix.PollFd{Fd: int32(fd), Events: unix.POLLIN})

		// Connection fds
		for _, conn := range fd2conn {
			if conn == nil {
				continue
			}
			var events int16
			if conn.state == StateReq {
				events = unix.POLLIN
			} else {
				events = unix.POLLOUT
			}
			events = events | unix.POLLERR
			pollArgs = append(pollArgs, unix.PollFd{Fd: int32(conn.fd), Events: events})
		}

		// Poll for active fds
		_, err := unix.Poll(pollArgs, 1000)
		if err != nil {
			util.Die("poll", err)
		}

		// Process active connections
		for i := 1; i < len(pollArgs); i++ {
			if pollArgs[i].Revents != 0 {
				conn := fd2conn[pollArgs[i].Fd]
				connectionIO(conn)
				if conn.state == StateEnd {

					fmt.Println("conn.state == StateEnd")
					// Client closed normally, or something bad happened.
					// Destroy this connection
					fd2conn[conn.fd] = nil
					_ = syscall.Close(conn.fd)
				}
			}
		}

		// Try to accept a new connection if the listening fd is active
		if pollArgs[0].Revents != 0 {
			_ = acceptNewConn(&fd2conn, fd)
		}
	}
}

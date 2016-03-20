package main

import (
	"encoding/binary"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"github.com/kr/pty"
	"golang.org/x/crypto/ssh"
)

func startSession(newChannel ssh.NewChannel) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		log.Fatal("Could not accept channel: %v", err)
	}

	shell := exec.Command(defaultShell)

	teardown := func() {
		channel.Close()
		_, err := shell.Process.Wait()
		if err != nil {
			log.Printf("Failed to exit shell: %v", err)
		}
		log.Print("Session closed")
		os.Exit(0)
	}

	shellf, err := pty.Start(shell)
	if err != nil {
		log.Fatalf("Failed to spawn shell: %v", err)
	}

	// connect the pipes
	var once sync.Once
	go func() {
		io.Copy(channel, shellf)
		once.Do(teardown)
	}()
	go func() {
		io.Copy(shellf, channel)
		once.Do(teardown)
	}()

	go func(in <-chan *ssh.Request) {
		for req := range in {
			ok := false
			switch req.Type {
			case "shell":
				ok = true
				if len(req.Payload) > 0 {
					// only the default shell
					// is allowed
					ok = false
				}
			case "window-change":
				ok = true
				ws := parseWinSize(req.Payload)
				setWinSize(shellf.Fd(), ws)
			}
			req.Reply(ok, nil)
		}
	}(requests)
}

type winSize struct {
	height uint16
	width  uint16
	x      uint16
	y      uint16
}

// parse window size from payload
func parseWinSize(b []byte) *winSize {
	ws := &winSize{
		width:  uint16(binary.BigEndian.Uint32(b)),
		height: uint16(binary.BigEndian.Uint32(b[4:])),
	}
	return ws
}

// update window size of a pty
func setWinSize(fd uintptr, ws *winSize) {
	syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}

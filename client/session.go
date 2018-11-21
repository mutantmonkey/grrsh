package main

import (
	"encoding/binary"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/kr/pty"
	"golang.org/x/crypto/ssh"
)

func startSession(newChannel ssh.NewChannel) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		log.Fatalf("Could not accept channel: %v", err)
	}

	var pt *os.File
	var tt *os.File

	go func(in <-chan *ssh.Request) {
		for req := range in {
			ok := false
			switch req.Type {
			case "pty-req":
				ok = true
				pt, tt, err = pty.Open()
				if err != nil {
					log.Printf("Failed to open PTY: %v", err)
					return
				}
				ws := parsePtyReq(req.Payload)
				pty.Setsize(pt, ws)
			case "shell":
				ok = true
				if len(req.Payload) > 0 {
					// only the default shell
					// is allowed
					ok = false
				}

				err = spawnShell(pt, tt, channel)
				if err != nil {
					log.Print("Failed to spawn shell: %v", err)
					ok = false
				}
			case "window-change":
				ok = true
				ws := parseWinChange(req.Payload)
				pty.Setsize(pt, ws)
			}
			req.Reply(ok, nil)
		}
	}(requests)
}

func spawnShell(pt *os.File, tt *os.File, channel ssh.Channel) (err error) {
	shell := exec.Command(defaultShell)

	teardown := func() {
		channel.Close()

		// wait for shell to terminate on a best effort basis
		// this can fail if the process has already terminated, but we
		// don't care, so errors are ignored
		_, _ = shell.Process.Wait()

		pt.Close()
	}

	if pt != nil && tt != nil {
		shell.Stdout = tt
		shell.Stdin = tt
		shell.Stderr = tt
		if shell.SysProcAttr == nil {
			shell.SysProcAttr = &syscall.SysProcAttr{}
		}
		shell.SysProcAttr.Setctty = true
		shell.SysProcAttr.Setsid = true
		err = shell.Start()
	} else {
		pt, err = pty.Start(shell)
	}

	if err != nil {
		return
	}

	// connect the pipes
	var once sync.Once
	go func() {
		io.Copy(channel, pt)
		once.Do(teardown)
	}()
	go func() {
		io.Copy(pt, channel)
		once.Do(teardown)
	}()

	shell.Process.Wait()
	once.Do(teardown)

	return nil
}

func parsePtyReq(b []byte) *pty.Winsize {
	termLength := uint32(binary.BigEndian.Uint32(b))
	//term := b[4:termLength + 4]
	offset := 4 + termLength

	ws := &pty.Winsize{
		Rows: uint16(binary.BigEndian.Uint32(b[offset+12 : offset+16])),
		Cols: uint16(binary.BigEndian.Uint32(b[offset+8 : offset+12])),
		X:    uint16(binary.BigEndian.Uint32(b[offset+8 : offset+12])),
		Y:    uint16(binary.BigEndian.Uint32(b[offset+12 : offset+16])),
	}
	return ws
}

// parse window size from payload
func parseWinChange(b []byte) *pty.Winsize {
	ws := &pty.Winsize{
		Rows: uint16(binary.BigEndian.Uint32(b[4:])),
		Cols: uint16(binary.BigEndian.Uint32(b)),
		X:    uint16(binary.BigEndian.Uint32(b[8:])),
		Y:    uint16(binary.BigEndian.Uint32(b[12:])),
	}
	return ws
}

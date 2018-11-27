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
			case "exec":
				ok = true
				cmdLen := binary.BigEndian.Uint32(req.Payload)
				cmdLine := string(req.Payload[4 : cmdLen+4])

				// Run the command using the default shell
				cmd := exec.Command(defaultShell, "-c", cmdLine)
				if tt != nil {
					err = startInPty(cmd, pt, tt, channel)
					if err != nil {
						log.Printf("Failed to execute command: %v", err)
						ok = false
					}
				} else {
					cmd.Stdin = channel
					cmd.Stdout = channel
					cmd.Stderr = channel
					err := cmd.Start()
					if err != nil {
						log.Printf("Failed to run command: %v", err)
						channel.Close()
						ok = false
					} else {
						ok = true
						go func() {
							cmd.Process.Wait()
							channel.Close()
						}()
					}
				}
			case "pty-req":
				ok = true
				pt, tt, err = pty.Open()
				if err != nil {
					log.Printf("Failed to open PTY: %v", err)
					ok = false
				} else {
					ws := parsePtyReq(req.Payload)
					pty.Setsize(pt, ws)
				}
			case "shell":
				ok = true
				if len(req.Payload) > 0 {
					// only the default shell
					// is allowed
					ok = false
				}

				if pt == nil || tt == nil {
					pt, tt, err = pty.Open()
					if err != nil {
						log.Printf("Failed to open PTY: %v", err)
						ok = false
					}
				}

				err = spawnShell(pt, tt, channel)
				if err != nil {
					log.Printf("Failed to spawn shell: %v", err)
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

func startInPty(cmd *exec.Cmd, pt *os.File, tt *os.File, channel ssh.Channel) (err error) {
	cmd.Stdout = tt
	cmd.Stdin = tt
	cmd.Stderr = tt
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setctty = true
	cmd.SysProcAttr.Setsid = true

	err = cmd.Start()

	teardown := func() {
		channel.Close()

		// wait for command to terminate on a best effort basis
		// this can fail if the process has already terminated, but we
		// don't care, so errors are ignored
		_, _ = cmd.Process.Wait()

		pt.Close()
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
	go func() {
		cmd.Process.Wait()
		once.Do(teardown)
	}()

	return
}

func spawnShell(pt *os.File, tt *os.File, channel ssh.Channel) error {
	shell := exec.Command(defaultShell)
	return startInPty(shell, pt, tt, channel)
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
	var ws *pty.Winsize
	if len(b) > 8 {
		ws = &pty.Winsize{
			Rows: uint16(binary.BigEndian.Uint32(b[4:])),
			Cols: uint16(binary.BigEndian.Uint32(b)),
			X:    uint16(binary.BigEndian.Uint32(b[8:])),
			Y:    uint16(binary.BigEndian.Uint32(b[12:])),
		}
	} else {
		ws = &pty.Winsize{
			Rows: uint16(binary.BigEndian.Uint32(b[4:])),
			Cols: uint16(binary.BigEndian.Uint32(b)),
		}
	}
	return ws
}

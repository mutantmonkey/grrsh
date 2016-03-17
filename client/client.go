package main

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/kr/pty"
	"golang.org/x/crypto/ssh"
)

// compare received server key with configured server key
func checkServerKey(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	receivedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))

	if receivedKey == serverPublicKey {
		return nil, nil
	} else {
		return nil, errors.New("Server key does not match")
	}
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

func main() {
	config := &ssh.ServerConfig{
		PublicKeyCallback: checkServerKey,
	}

	private, err := ssh.ParsePrivateKey([]byte(clientPrivateKey))
	if err != nil {
		log.Fatalf("Failed to parse client private key: %v", err)
	}

	config.AddHostKey(private)

	c, err := net.Dial("tcp", serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect to %q: %v", serverAddr, err)
	}

	_, chans, reqs, err := ssh.NewServerConn(c, config)
	if err != nil {
		log.Fatalf("Failed to handshake: %v", err)
	}

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

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
}

package main

import (
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/kr/pty"
	"golang.org/x/crypto/ssh"
)

func checkServerKey(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	receivedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))

	if receivedKey == serverPublicKey {
		return nil, nil
	} else {
		return nil, errors.New("Server key does not match")
	}
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

	// Service the incoming Channel channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of a shell, the type is
		// "session" and ServerShell may be used to present a simple
		// terminal interface.
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Fatal("Could not accept channel: %v", err)
		}

		// Sessions have out-of-band requests such as "shell",
		// "pty-req" and "env".  Here we handle only the
		// "shell" request.
		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := false
				switch req.Type {
				case "shell":
					ok = true
					if len(req.Payload) > 0 {
						// We don't accept any
						// commands, only the
						// default shell.
						ok = false
					}
				}
				req.Reply(ok, nil)
			}
		}(requests)

		go func() {
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
		}()
	}
}

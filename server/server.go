package main

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

func checkClientKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
	receivedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))

	if receivedKey == clientPublicKey {
		return nil
	} else {
		return errors.New("Client key does not match")
	}
}

func requestWindowChange(session *ssh.Session) {
	width, height, err := terminal.GetSize(0)
	if err != nil {
		return
	}

	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload, uint32(width))
	binary.BigEndian.PutUint32(payload[4:], uint32(height))

	session.SendRequest("window-change", true, payload)
}

func main() {
	private, err := ssh.ParsePrivateKey([]byte(serverPrivateKey))
	if err != nil {
		log.Fatalf("Failed to parse client private key: %v", err)
	}

	auths := []ssh.AuthMethod{
		ssh.PublicKeys(private),
	}

	config := &ssh.ClientConfig{
		User:            "user",
		Auth:            auths,
		HostKeyCallback: checkClientKey,
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %q: %v", listenAddr, err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Failed to accept incoming connection: %v", err)
			continue
		}

		log.Printf("New connection from %v", conn.RemoteAddr())

		sshConn, chans, reqs, err := ssh.NewClientConn(conn, "addr", config)
		if err != nil {
			log.Printf("Failed to handshake: %v", err)
			continue
		}

		client := ssh.NewClient(sshConn, chans, reqs)
		session, err := client.NewSession()
		if err != nil {
			log.Printf("Failed to create session: %v", err)
			continue
		}
		defer session.Close()

		oldState, err := terminal.MakeRaw(0)
		if err != nil {
			log.Printf("Failed to set terminal to raw mode")
			continue
		}
		defer terminal.Restore(0, oldState)
		requestWindowChange(session)

		// handle terminal resizes
		resizeChan := make(chan os.Signal)
		go func() {
			for _ = range resizeChan {
				requestWindowChange(session)
			}
		}()
		signal.Notify(resizeChan, syscall.SIGWINCH)

		session.Stderr = os.Stderr
		session.Stdin = os.Stdin
		session.Stdout = os.Stdout

		if err := session.Shell(); err != nil {
			log.Printf("Failed to run shell: %v", err)
		}

		session.Wait()
		terminal.Restore(0, oldState)
	}
}

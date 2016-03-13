package main

import (
	"log"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
)

const listenAddr = ":31337"

func main() {
	auths := []ssh.AuthMethod{
		ssh.Password("123456"),
	}

	config := &ssh.ClientConfig{
		User: "user",
		Auth: auths,
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on 31337: %v", err)
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

		session.Stderr = os.Stderr
		session.Stdin = os.Stdin
		session.Stdout = os.Stdout

		if err := session.Shell(); err != nil {
			log.Printf("Failed to run shell: %v", err)
		}
	}
}

package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

// RFC 4254 7.2
type channelOpenDirectMsg struct {
	Raddr string
	Rport uint32
	Laddr string
	Lport uint32
}

func startDirectTcpip(newChannel ssh.NewChannel) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel: %v", err)
		return
	}

	go ssh.DiscardRequests(requests)

	p := &channelOpenDirectMsg{}
	err = ssh.Unmarshal(newChannel.ExtraData(), p)
	if err != nil {
		log.Printf("Could not unmarshal extra data: %v", err)
		return
	}

	conn, err := net.Dial("tcp", net.JoinHostPort(p.Raddr, fmt.Sprint(p.Rport)))
	if err != nil {
		log.Printf("Failed to connect: %v", err)
		return
	}

	teardown := func() {
		channel.Close()
		conn.Close()
	}

	// connect the pipes
	var once sync.Once
	go func() {
		io.Copy(channel, conn)
		once.Do(teardown)
	}()
	go func() {
		io.Copy(conn, channel)
		once.Do(teardown)
	}()
}

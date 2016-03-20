package main

import (
	"errors"
	"log"
	"net"
	"strings"

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
		switch newChannel.ChannelType() {
		case "direct-tcpip":
			startDirectTcpip(newChannel)
		case "session":
			startSession(newChannel)
		default:
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
		}
	}
}

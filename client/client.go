package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/url"

	"github.com/cenkalti/backoff"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

// compare received server key with configured server key
func checkServerKey(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	checkKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(serverPublicKey))
	if err != nil {
		return nil, err
	}

	if bytes.Equal(key.Marshal(), checkKey.Marshal()) {
		return nil, nil
	} else {
		return nil, fmt.Errorf("ssh: server key mismatch")
	}
}

func prepareProxyDialer(proxyUrl string) (dialer proxy.Dialer, err error) {
	dialer = proxy.Direct

	if len(proxyUrl) > 0 {
		u, err := url.Parse(proxyUrl)
		if err != nil {
			return dialer, err
		}

		dialer, err = proxy.FromURL(u, dialer)
		if err != nil {
			return dialer, err
		}
	}

	return
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

	dialer, err := prepareProxyDialer(clientProxyUrl)
	if err != nil {
		log.Fatalf("Failed to create proxy dialer: %v", err)
	}

	for {
		var c net.Conn
		dialOp := func() (err error) {
			c, err = dialer.Dial("tcp", serverAddr)
			return err
		}

		backoff.Retry(dialOp, backoff.NewExponentialBackOff())
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
}

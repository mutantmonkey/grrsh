package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

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

type hostport string

func (h *hostport) String() string {
	return fmt.Sprint(*h)
}

func (h *hostport) Set(value string) error {
	s := strings.Split(value, ":")
	if len(s) == 1 {
		*h = hostport(fmt.Sprintf("localhost:%s", s[0]))
	} else if len(s) == 2 {
		*h = hostport(value)
	} else {
		return errors.New("invalid dynamic forwarding specification")
	}

	return nil
}

type remoteForwards []forwardPair

func (r *remoteForwards) String() string {
	return fmt.Sprint(*r)
}

func (r *remoteForwards) Set(value string) error {
	f := forwardPair{}
	s := strings.Split(value, ":")
	if len(s) == 3 {
		f.laddr = fmt.Sprintf("localhost:%s", s[0])
		f.raddr = fmt.Sprintf("%s:%s", s[1], s[2])
	} else if len(s) == 4 {
		f.laddr = fmt.Sprintf("%s:%s", s[0], s[1])
		f.raddr = fmt.Sprintf("%s:%s", s[2], s[3])
	} else {
		return errors.New("invalid forwarding specification")
	}

	*r = append(*r, f)
	return nil
}

func main() {
	var socksFlag hostport
	var remoteForwardsFlag remoteForwards
	var listenersOnly bool
	flag.Var(&socksFlag, "D", "[bind_address:]port")
	flag.Var(&remoteForwardsFlag, "L", "[bind_address:]port:host:hostport")
	flag.BoolVar(&listenersOnly, "z", false, "")
	flag.Parse()

	if listenersOnly {
		log.Print("Local listeners only mode: each remote connection will open a local port")
	}

	private, err := ssh.ParsePrivateKey([]byte(serverPrivateKey))
	if err != nil {
		log.Fatalf("Failed to parse server private key: %v", err)
	}

	auths := []ssh.AuthMethod{
		ssh.PublicKeys(private),
	}

	hostKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(clientPublicKey))
	if err != nil {
		log.Fatalf("Failed to parse client public key: %v", err)
	}

	config := &ssh.ClientConfig{
		User:            "user",
		Auth:            auths,
		HostKeyCallback: ssh.FixedHostKey(hostKey),
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

		if listenersOnly {
			localLn, err := net.Listen("tcp", "[::1]:")
			if err != nil {
				log.Printf("Failed to open local listener: %v", err)
				continue
			}

			log.Printf("Local listener for %v opened at %v", conn.RemoteAddr(), localLn.Addr())

			// TODO: close the listener when the remote disconnects
			// use waitgroups maybe?
			// XXX: there's actually a problem on the client where
			// it kills everything too early

			go func(c net.Conn, l net.Listener) {
				for {
					localConn, err := l.Accept()
					if err != nil {
						log.Printf("Failed to accept incoming local connection: %v", err)
						continue
					}

					go func(rc net.Conn, lc net.Conn) {
						teardown := func() {
							lc.Close()
						}

						// connect the pipes
						var once sync.Once
						go func() {
							io.Copy(rc, lc)
							once.Do(teardown)
						}()

						io.Copy(lc, rc)
						once.Do(teardown)
					}(c, localConn)
				}

				/*wg.Wait()
				l.Close()
				conn.Close()*/
			}(conn, localLn)

			// TODO: close listener and remote connection when all
			// local connections are closed
		} else {
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

			closedChannel := make(chan string)

			if len(socksFlag) > 0 {
				log.Printf("Forward SOCKS5 traffic on %v", socksFlag)
				go func() {
					err = socksListen(client, closedChannel, string(socksFlag))
					if err != nil {
						log.Printf("Failed to open SOCKS5 listener: %v", err)
					}
				}()
			}

			for _, forward := range remoteForwardsFlag {
				log.Printf("Forward remote %v to local %v", forward.raddr, forward.laddr)
				go func() {
					err = forwardRemote(client, closedChannel, forward)
					if err != nil {
						log.Printf("Failed to forward remote port: %v", err)
					}
				}()
			}

			session.Wait()
			terminal.Restore(0, oldState)

			go func() {
				closedChannel <- "close"
			}()
		}
	}
}

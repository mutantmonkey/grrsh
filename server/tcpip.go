package main

import (
	"io"
	"log"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
	"mutantmonkey.in/code/grrsh/server/socks"
)

type forwardPair struct {
	laddr string
	raddr string
}

func socksListen(client *ssh.Client, closedChannel <-chan string, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		<-closedChannel
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		defer conn.Close()

		go func(conn net.Conn) {
			err := socks.Handshake(conn)
			if err != nil {
				log.Printf("Failed to SOCKS handshake: %v", err)
				conn.Close()
				return
			}

			request, err := socks.UnmarshalRequest(conn)
			if err != nil {
				log.Printf("Failed to unmarshal SOCKS request: %v", err)
				socks.SendReply(conn, socks.ReplyGeneralFailure)
				conn.Close()
				return
			}

			if request.Command != socks.CmdConnect {
				socks.SendReply(conn, socks.ReplyCommandNotSupported)
				conn.Close()
				return
			}

			rconn, err := client.Dial("tcp", request.Addr.String())
			if err != nil {
				log.Printf("%v", err)
				socks.SendReply(conn, socks.ReplyGeneralFailure)
				conn.Close()
				return
			}

			if rconn != nil {
				socks.SendReply(conn, socks.ReplySuccess)

				teardown := func() {
					conn.Close()
					rconn.Close()
				}

				// connect the pipes
				var once sync.Once
				go func() {
					io.Copy(conn, rconn)
					once.Do(teardown)
				}()
				go func() {
					io.Copy(rconn, conn)
					once.Do(teardown)
				}()
				go func() {
					<-closedChannel
					once.Do(teardown)
				}()
			}
		}(conn)
	}
}

func forwardRemote(client *ssh.Client, closedChannel <-chan string, forward forwardPair) error {
	ln, err := net.Listen("tcp", forward.laddr)
	if err != nil {
		return err
	}

	go func() {
		<-closedChannel
		ln.Close()
	}()

	for {
		lconn, err := ln.Accept()
		if err != nil {
			return err
		}

		go func() {
			rconn, err := client.Dial("tcp", forward.raddr)
			if err != nil {
				log.Printf("Unable to dial: %v", err)
				lconn.Close()
			}
			if rconn != nil {
				teardown := func() {
					lconn.Close()
					rconn.Close()
				}

				// connect the pipes
				var once sync.Once
				go func() {
					io.Copy(lconn, rconn)
					once.Do(teardown)
				}()
				go func() {
					io.Copy(rconn, lconn)
					once.Do(teardown)
				}()
				go func() {
					<-closedChannel
					once.Do(teardown)
				}()
			}
		}()
	}

	return nil
}

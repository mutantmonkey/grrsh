package main

import (
	"io"
	"log"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

type forwardPair struct {
	laddr string
	raddr string
}

func forwardRemote(client *ssh.Client, messages <-chan string, forward forwardPair) error {
	ln, err := net.Listen("tcp", forward.laddr)
	if err != nil {
		return err
	}

	go func() {
		<-messages
		ln.Close()
	}()

	for {
		lconn, err := ln.Accept()
		if err != nil {
			return err
		}

		if lconn != nil {
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
						<-messages
						once.Do(teardown)
					}()
				}
			}()
		}
	}

	return nil
}

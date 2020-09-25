// Copyright 2020 Lennart Espe
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"io"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
)

type LoadBalancer interface {
	Next(incoming net.Addr) string
}

type RoundRobin struct {
	Origins []string
	mu      sync.Mutex
	p       int
}

func (rr *RoundRobin) Next(incoming net.Addr) string {
	rr.mu.Lock()
	c := rr.p
	rr.p++
	rr.mu.Unlock()
	return rr.Origins[c]
}

type Proxy struct {
	Name       string
	Addr       string
	MaxRetries int
	LB         LoadBalancer
}

func (proxy *Proxy) Listen(ctx context.Context) {
	// Open TCP endpoint
	listener, err := net.Listen("tcp", proxy.Addr)
	if err != nil {
		logger.WithField("name", proxy.Name).WithError(err).Error("Could not listen on TCP addr")
		return
	}
	logger.WithFields(logrus.Fields{
		"name": proxy.Name,
		"addr": proxy.Addr,
	}).Debug("Waiting for incoming connections")
	// Shutdown listener when context closes
	go func() {
		<-ctx.Done()
		if err := listener.Close(); err != nil {
			logger.WithField("name", proxy.Name).WithError(err).Warn("Could not shutdown listener")
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go proxy.Forward(ctx, conn)
	}
}

func (proxy *Proxy) Forward(ctx context.Context, incoming net.Conn) {
	var (
		endpoint net.Conn
		origin   = proxy.LB.Next(incoming.RemoteAddr())
		trial    = 0
	)
	defer incoming.Close()
	// Attempt to open tcp connection
	for endpoint == nil {
		ep, err := net.Dial("tcp", origin)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"name":   proxy.Name,
				"origin": origin,
				"trial":  trial,
				"remote": incoming.RemoteAddr(),
			}).WithError(err).Warn("Could not reach origin endpoint")
			if trial > proxy.MaxRetries {
				return
			}
			trial++
		} else {
			endpoint = ep
			defer endpoint.Close()
		}
	}
	logger.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"origin":   origin,
		"edge":     incoming.RemoteAddr(),
		"endpoint": endpoint.RemoteAddr(),
	}).Debug("Forwarding connection to endpoint")
	// Now we have a working endpoint
	ch := make(chan struct{}, 2)
	go func() { io.Copy(endpoint, incoming); ch <- struct{}{} }()
	go func() { io.Copy(incoming, endpoint); ch <- struct{}{} }()
	<-ch
	<-ch
	logger.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"origin":   origin,
		"edge":     incoming.RemoteAddr(),
		"endpoint": endpoint.RemoteAddr(),
	}).Debug("Connection got terminated")
}

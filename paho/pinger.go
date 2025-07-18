/*
 * Copyright (c) 2024 Contributors to the Eclipse Foundation
 *
 *  All rights reserved. This program and the accompanying materials
 *  are made available under the terms of the Eclipse Public License v2.0
 *  and Eclipse Distribution License v1.0 which accompany this distribution.
 *
 * The Eclipse Public License is available at
 *    https://www.eclipse.org/legal/epl-2.0/
 *  and the Eclipse Distribution License is available at
 *    http://www.eclipse.org/org/documents/edl-v10.php.
 *
 *  SPDX-License-Identifier: EPL-2.0 OR BSD-3-Clause
 */

package paho

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rtalhouk/paho.golang/packets"
	"github.com/rtalhouk/paho.golang/paho/log"
)

type Pinger interface {
	// Run starts the pinger. It blocks until the pinger is stopped.
	// If the pinger stops due to an error, it returns the error.
	// If the keepAlive is 0, it returns nil immediately.
	// Run() may be called multiple times, but only after prior instances have terminated.
	Run(ctx context.Context, conn net.Conn, keepAlive uint16) error

	// PacketSent is called when a packet is sent to the server.
	PacketSent()

	// PacketReceived is called when a packet (possibly a PINGRESP) is received from the server
	PacketReceived()

	// PingResp is called when a PINGRESP is received from the server.
	PingResp()

	// SetDebug sets the logger for debugging.
	// It is not thread-safe and must be called before Run() to avoid race conditions.
	SetDebug(log.Logger)
}

// DefaultPinger is the default implementation of Pinger.
type DefaultPinger struct {
	lastPacketSent     time.Time
	lastPacketReceived time.Time
	lastPingResponse   time.Time

	debug log.Logger

	running bool // Used to prevent concurrent calls to Run

	mu sync.Mutex // Protects all of the above
}

// NewDefaultPinger creates a DefaultPinger
func NewDefaultPinger() *DefaultPinger {
	return &DefaultPinger{
		debug: log.NOOPLogger{},
	}
}

// Run starts the pinger; blocks until done (either context cancelled or error encountered)
func (p *DefaultPinger) Run(ctx context.Context, conn net.Conn, keepAlive uint16) error {
	if keepAlive == 0 {
		p.debug.Println("Run() returning immediately due to keepAlive == 0")
		return nil
	}
	if conn == nil {
		return fmt.Errorf("conn is nil")
	}
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("Run() already in progress")
	}
	p.running = true
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	interval := time.Duration(keepAlive) * time.Second
	timer := time.NewTimer(0) // Immediately send first pingreq
	// If timer is not stopped, it cannot be garbage collected until it fires.
	defer timer.Stop()
	var lastPingSent time.Time
	// errCh should be buffered, so that the goroutine sending the error does not block if the context is cancelled
	errCh := make(chan error, 1)
	for {
		select {
		case <-ctx.Done():
			return nil
		case t := <-timer.C:
			p.mu.Lock()
			lastPingResponse := p.lastPingResponse
			// The MQTT Spec only requires that a ping be sent if no control packets have been SENT within the keepalive
			// period (MQTT-3.1.2-20). Only sending PING in that one case can cause issues if the only activity is
			// outgoing messages, a half-open connection should result in a TCP timeout but this can take a long time
			// (issue #288). To address this we PING if we have not both sent, and received, packets within keepAlive.
			var pingDue time.Time
			if p.lastPacketSent.Before(p.lastPacketReceived) {
				pingDue = p.lastPacketSent.Add(interval)
			} else {
				pingDue = p.lastPacketReceived.Add(interval)
			}
			p.mu.Unlock()

			if !lastPingSent.IsZero() && lastPingSent.After(lastPingResponse) {
				p.debug.Printf("DefaultPinger PINGRESP timeout")
				return fmt.Errorf("PINGRESP timed out")
			}

			if t.Before(pingDue) {
				// A Control Packet has been sent since we last checked, meaning the ping can be delayed
				timer.Reset(pingDue.Sub(t))
				continue
			}
			lastPingSent = time.Now()
			go func() {
				// WriteTo may not complete within KeepAlive period due to slow/unstable network.
				// For instance, if a huge message is sent over a very slow link at the same time as PINGREQ packet,
				// the Write operation may block for longer than KeepAlive interval.
				// Note: connection closure unblocks the Write operation. So, the goroutine is not leaked.
				if _, err := packets.NewControlPacket(packets.PINGREQ).WriteTo(conn); err != nil {
					p.debug.Printf("DefaultPinger packet write error: %v", err)
					errCh <- fmt.Errorf("failed to send PINGREQ: %w", err)
				}
			}()
			timer.Reset(interval)
		case err := <-errCh:
			return err
		}
	}
}

func (p *DefaultPinger) PacketSent() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastPacketSent = time.Now()
}

func (p *DefaultPinger) PacketReceived() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastPacketReceived = time.Now()
}

func (p *DefaultPinger) PingResp() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastPingResponse = time.Now()
}

func (p *DefaultPinger) SetDebug(debug log.Logger) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.debug = debug
}

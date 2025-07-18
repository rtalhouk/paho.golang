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

import "github.com/rtalhouk/paho.golang/packets"

type (
	// Auth is a representation of the MQTT Auth packet
	Auth struct {
		Properties *AuthProperties
		ReasonCode byte
	}

	// AuthProperties is a struct of the properties that can be set
	// for a Auth packet
	AuthProperties struct {
		AuthData     []byte
		AuthMethod   string
		ReasonString string
		User         UserProperties
	}
)

// InitProperties is a function that takes a lower level
// Properties struct and completes the properties of the Auth on
// which it is called
func (a *Auth) InitProperties(p *packets.Properties) {
	a.Properties = &AuthProperties{
		AuthMethod:   p.AuthMethod,
		AuthData:     p.AuthData,
		ReasonString: p.ReasonString,
		User:         UserPropertiesFromPacketUser(p.User),
	}
}

// AuthFromPacketAuth takes a packets library Auth and
// returns a paho library Auth
func AuthFromPacketAuth(a *packets.Auth) *Auth {
	v := &Auth{ReasonCode: a.ReasonCode}
	v.InitProperties(a.Properties)

	return v
}

// Packet returns a packets library Auth from the paho Auth
// on which it is called
func (a *Auth) Packet() *packets.Auth {
	v := &packets.Auth{ReasonCode: a.ReasonCode}

	if a.Properties != nil {
		v.Properties = &packets.Properties{
			AuthMethod:   a.Properties.AuthMethod,
			AuthData:     a.Properties.AuthData,
			ReasonString: a.Properties.ReasonString,
			User:         a.Properties.User.ToPacketProperties(),
		}
	}

	return v
}

// AuthResponse is a represenation of the response to an Auth
// packet
type AuthResponse struct {
	Properties *AuthProperties
	ReasonCode byte
	Success    bool
}

// AuthResponseFromPacketAuth takes a packets library Auth and
// returns a paho library AuthResponse
func AuthResponseFromPacketAuth(a *packets.Auth) *AuthResponse {
	return &AuthResponse{
		Success:    true,
		ReasonCode: a.ReasonCode,
		Properties: &AuthProperties{
			ReasonString: a.Properties.ReasonString,
			User:         UserPropertiesFromPacketUser(a.Properties.User),
		},
	}
}

// AuthResponseFromPacketDisconnect takes a packets library Disconnect and
// returns a paho library AuthResponse
func AuthResponseFromPacketDisconnect(d *packets.Disconnect) *AuthResponse {
	return &AuthResponse{
		Success:    true,
		ReasonCode: d.ReasonCode,
		Properties: &AuthProperties{
			ReasonString: d.Properties.ReasonString,
			User:         UserPropertiesFromPacketUser(d.Properties.User),
		},
	}
}

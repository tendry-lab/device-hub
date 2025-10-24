/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package sysnet

import "net"

// ResolveHandler to handle the result of network address resolving.
type ResolveHandler interface {
	// HandleResolve handles the resolving result of hostname to addr.
	HandleResolve(hostname string, addr net.Addr)
}

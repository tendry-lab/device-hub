/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package syssched

// FuncStopper is a function type that implements the Stopper interface.
type FuncStopper func() error

// Stop calls the function itself to fulfill the Stopper interface.
func (s FuncStopper) Stop() error {
	return s()
}

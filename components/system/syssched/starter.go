/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package syssched

// Starter is responsible for starting an execution.
type Starter interface {
	// Start starts an execution.
	Start() error
}

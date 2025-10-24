/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package syssched

// Stopper implementation should free all allocated resources.
type Stopper interface {
	// Stop stops the resource.
	Stop() error
}

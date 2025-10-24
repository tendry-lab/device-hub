/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package syssched

// Awakener to wake up an execution.
type Awakener interface {
	// Awake wakes up an execution.
	Awake()
}

/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package syssched

// Task represents an entity of the execution.
type Task interface {
	// Run executes a single operational loop.
	Run() error
}

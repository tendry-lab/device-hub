/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package syssched

// ErrorHandler handles errors.
type ErrorHandler interface {
	// HandleError handles error.
	HandleError(err error)
}

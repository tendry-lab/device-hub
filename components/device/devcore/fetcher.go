/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package devcore

// Fetcher fetches the device data from the arbitrary source.
type Fetcher interface {
	// Fetch the device data.
	Fetch() ([]byte, error)
}

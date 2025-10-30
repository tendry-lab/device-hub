/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package devcore

// TimeSynchronizer synchronizes time between local and remote resources.
type TimeSynchronizer interface {
	// SyncTime synchronizes the UNIX time for a device.
	SyncTime() error
}

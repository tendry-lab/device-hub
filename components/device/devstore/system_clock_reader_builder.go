/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package devstore

import (
	"github.com/tendry-lab/device-hub/components/storage/stcore"
)

// SystemClockReaderBuilder - system clock reader builder.
type SystemClockReaderBuilder interface {
	// BuildReader builds system clock reader for the device.
	BuildReader(deviceID string) stcore.SystemClockReader
}

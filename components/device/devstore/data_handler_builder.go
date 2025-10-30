/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package devstore

import (
	"github.com/tendry-lab/device-hub/components/device/devcore"
	"github.com/tendry-lab/device-hub/components/system/syscore"
)

// DataHandlerBuilder - data handler builder.
type DataHandlerBuilder interface {
	// BuildHandler builds a data handler that updates the provided system clock
	// each time it receives data.
	BuildHandler(clock syscore.SystemClock, deviceID string) devcore.DataHandler
}

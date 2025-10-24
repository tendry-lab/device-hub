/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package devstore

import "github.com/tendry-lab/device-hub/components/system/syssched"

// AliveMonitor to monitor the device well-being.
type AliveMonitor interface {
	// Monitor monitors the device well-being.
	//
	// Remarks:
	//	- If the device isn't marked as alive with the returned alive notifier,
	//	  it can be considered as inactive.
	Monitor(uri string) syssched.AliveNotifier
}

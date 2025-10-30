/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package syscore

import "time"

// MonotonicClock to read monotonic time.
type MonotonicClock interface {
	// Now returns a monotonic clock reading.
	Now() time.Time
}

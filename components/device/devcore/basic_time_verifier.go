/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package devcore

// BasicTimeVerifier ensures that the device UNIX time is greater than 0.
type BasicTimeVerifier struct{}

// VerifyTime returns true if the device UNIX time is greater than 0.
func (*BasicTimeVerifier) VerifyTime(timestamp int64) bool {
	return timestamp > 0
}

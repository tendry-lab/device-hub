/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package devcore

import "sync"

// IDHolder holds the device unique identifier.
type IDHolder struct {
	mu sync.RWMutex
	id string
}

// NewIDHolder is an initialization of IDHolder.
func NewIDHolder() *IDHolder {
	return &IDHolder{}
}

// Get returns the unique device identifier.
//
// Remarks:
//   - Can be used by multiple goroutines.
func (h *IDHolder) Get() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.id
}

// Set updates the device ID.
//
// Remarks:
//   - Should be used by a single goroutine.
//   - Device ID is changed very rarely, rw-lock is used to reduce contention.
func (h *IDHolder) Set(deviceID string) {
	h.mu.RLock()
	id := h.id
	h.mu.RUnlock()

	if id != deviceID {
		h.mu.Lock()
		h.id = deviceID
		h.mu.Unlock()
	}
}

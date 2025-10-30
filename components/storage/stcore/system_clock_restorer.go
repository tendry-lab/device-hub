/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package stcore

import (
	"context"
	"sync"

	"github.com/tendry-lab/device-hub/components/status"
	"github.com/tendry-lab/device-hub/components/system/syscore"
)

// SystemClockRestorer restores the UNIX timestamp from the persistent storage.
type SystemClockRestorer struct {
	ctx    context.Context
	reader SystemClockReader

	mu        sync.Mutex
	restored  bool
	timestamp int64
}

// NewSystemClockRestorer is an initialization of SystemClockRestorer.
func NewSystemClockRestorer(
	ctx context.Context,
	reader SystemClockReader,
) *SystemClockRestorer {
	return &SystemClockRestorer{
		ctx:       ctx,
		reader:    reader,
		timestamp: int64(-1),
	}
}

// SetTimestamp sets the most recent UNIX time.
func (r *SystemClockRestorer) SetTimestamp(timestamp int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if timestamp > r.timestamp {
		r.timestamp = timestamp
	}

	if !r.restored {
		r.restored = true

		syscore.LogInf.Printf("skip timestamp restoring: value=%v", timestamp)
	}

	return nil
}

// GetTimestamp returns the most recent UNIX time.
func (r *SystemClockRestorer) GetTimestamp() (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.restored {
		return -1, status.StatusInvalidState
	}

	return r.timestamp, nil
}

// HandleError handles error from the Run() call.
func (*SystemClockRestorer) HandleError(err error) {
	if err != status.StatusNoData {
		syscore.LogErr.Printf("failed to restore timestamp: err=%v", err)
	}
}

// Run restores the UNIX timestamp from the persistent storage.
func (r *SystemClockRestorer) Run() error {
	timestamp, err := r.reader.ReadTimestamp(r.ctx)
	if err != nil && err != status.StatusNoData {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.restored {
		syscore.LogInf.Printf("timestamp already restored: restored=%v persisted=%v",
			r.timestamp, timestamp)
	} else {
		r.restored = true
		r.timestamp = timestamp

		syscore.LogInf.Printf("timestamp restored: value=%v", r.timestamp)
	}

	return nil
}

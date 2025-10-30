/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package devstore

import (
	"context"

	"github.com/tendry-lab/device-hub/components/device/devcore"
	"github.com/tendry-lab/device-hub/components/status"
	"github.com/tendry-lab/device-hub/components/storage/stcore"
)

type systemClockReader struct {
	holder  *devcore.IDHolder
	builder SystemClockReaderBuilder
	reader  stcore.SystemClockReader
}

func newSystemClockReader(
	holder *devcore.IDHolder,
	builder SystemClockReaderBuilder,
) *systemClockReader {
	return &systemClockReader{
		holder:  holder,
		builder: builder,
	}
}

func (r *systemClockReader) ReadTimestamp(ctx context.Context) (int64, error) {
	if r.reader == nil {
		deviceID := r.holder.Get()
		if deviceID == "" {
			return -1, status.StatusError
		}

		r.reader = r.builder.BuildReader(deviceID)
		if r.reader == nil {
			panic("invalid state: reader can't be nil")
		}
	}

	return r.reader.ReadTimestamp(ctx)
}

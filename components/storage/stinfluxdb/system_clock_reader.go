/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package stinfluxdb

import (
	"context"
	"fmt"
	"strings"

	"github.com/influxdata/influxdb-client-go/v2/api"

	"github.com/tendry-lab/device-hub/components/status"
	"github.com/tendry-lab/device-hub/components/system/syscore"
)

// SystemClockReaderParams - various parameters used during timestamp reading.
type SystemClockReaderParams struct {
	// Bucket - InfluxDB bucket name.
	Bucket string

	// DeviceID - device identifier.
	DeviceID string

	// TimestampRestoreRange - number of days to use for the timestamp lookup.
	TimestampRestoreRange int
}

// SystemClockReader reads the UNIX timestamp from the influxdb.
type SystemClockReader struct {
	params SystemClockReaderParams
	client api.QueryAPI
}

// NewSystemClockReader is an initialization of SystemClockReader.
func NewSystemClockReader(
	client api.QueryAPI,
	params SystemClockReaderParams,
) *SystemClockReader {
	return &SystemClockReader{
		params: params,
		client: client,
	}
}

// ReadTimestamp reads the most recent UNIX timestamp from the influxdb.
func (r *SystemClockReader) ReadTimestamp(ctx context.Context) (int64, error) {
	query := fmt.Sprintf(`
	from(bucket: "%s")
	  |> range(start: -%dd)
	  |> filter(fn: (r) => r["_measurement"] == "%s" and r["device_id"] == "%s")
	  |> last()
	  |> keep(columns: ["_time"])`,
		r.params.Bucket, r.params.TimestampRestoreRange, "telemetry", r.params.DeviceID)

	result, err := r.client.Query(ctx, query)
	if err != nil {
		syscore.LogErr.Printf("failed to perform query: %v", err)

		// HACK: library doesn't return the specific errors, so it's hard to tell what's wrong.
		if strings.Contains(err.Error(), "unauthorized") {
			return -1, status.StatusInvalidState
		}
		if strings.Contains(err.Error(), "not found") {
			return -1, status.StatusNoData
		}

		return -1, fmt.Errorf("influxdb: query failed: %w", err)
	}
	defer result.Close()

	if result.Err() != nil {
		return -1, result.Err()
	}

	if !result.Next() {
		if result.Err() != nil {
			return -1, fmt.Errorf("influxdb: query error: %w", result.Err())
		}

		return -1, status.StatusNoData
	}

	record := result.Record()
	if record == nil {
		return -1, fmt.Errorf("influxdb: no valid record returned")
	}

	timestamp := record.Time().Unix()

	syscore.LogInf.Printf("influxdb: read latest device UNIX timestamp: id=%v value=%v",
		r.params.DeviceID, timestamp)

	return timestamp, nil
}

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

// SystemClockReader reads the UNIX timestamp from the influxdb.
type SystemClockReader struct {
	bucket   string
	deviceID string
	client   api.QueryAPI
}

// NewSystemClockReader is an initialization of SystemClockReader.
func NewSystemClockReader(
	client api.QueryAPI,
	bucket string,
	deviceID string,
) *SystemClockReader {
	return &SystemClockReader{
		bucket:   bucket,
		deviceID: deviceID,
		client:   client,
	}
}

// ReadTimestamp reads the most recent UNIX timestamp from the influxdb.
func (r *SystemClockReader) ReadTimestamp(ctx context.Context) (int64, error) {
	query := fmt.Sprintf(`
	from(bucket: "%s")
	  |> range(start: -30d)
	  |> filter(fn: (r) => r["_measurement"] == "%s" and r["device_id"] == "%s")
	  |> last()
	  |> keep(columns: ["_time"])`, r.bucket, "telemetry", r.deviceID)

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
		r.deviceID, timestamp)

	return timestamp, nil
}

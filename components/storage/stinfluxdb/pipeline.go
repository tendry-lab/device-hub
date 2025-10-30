/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package stinfluxdb

import (
	"context"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"

	"github.com/tendry-lab/device-hub/components/device/devcore"
	"github.com/tendry-lab/device-hub/components/storage/stcore"
	"github.com/tendry-lab/device-hub/components/system/syscore"
)

// DBParams provides various configuration options for influxDB.
type DBParams struct {
	// URL - InfluxDB URL.
	URL string

	// Org - InfluxDB organisation name.
	Org string

	// Token - InfluxDB API token.
	Token string

	// Bucket - InfluxDB bucket name.
	Bucket string

	// TimestampRestoreRange - number of days to use for the timestamp lookup.
	TimestampRestoreRange int
}

// Pipeline contains various building blocks for persisting data in influxdb.
type Pipeline struct {
	params      DBParams
	ctx         context.Context
	dbClient    influxdb2.Client
	queryClient api.QueryAPI
	writeClient api.WriteAPIBlocking
}

// NewPipeline initializes all components associated with the influxdb subsystem.
//
// Parameters:
//   - ctx - parent context.
//   - params - various influxDB configuration parameters.
func NewPipeline(ctx context.Context, params DBParams) *Pipeline {
	dbClient := influxdb2.NewClient(params.URL, params.Token)
	writeClient := dbClient.WriteAPIBlocking(params.Org, params.Bucket)
	queryClient := dbClient.QueryAPI(params.Org)

	return &Pipeline{
		params:      params,
		ctx:         ctx,
		dbClient:    dbClient,
		queryClient: queryClient,
		writeClient: writeClient,
	}
}

// BuildReader builds reader that retrieves device timestamps from InfluxDB.
func (p *Pipeline) BuildReader(deviceID string) stcore.SystemClockReader {
	return NewSystemClockReader(p.queryClient, SystemClockReaderParams{
		Bucket:                p.params.Bucket,
		DeviceID:              deviceID,
		TimestampRestoreRange: p.params.TimestampRestoreRange,
	})
}

// BuildHandler builds the data handler that stores the device data in InfluxDB.
func (p *Pipeline) BuildHandler(
	clock syscore.SystemClock,
	_ string,
) devcore.DataHandler {
	return NewDataHandler(p.ctx, clock, p.writeClient)
}

// Stop stops writing data to the DB.
func (p *Pipeline) Stop() error {
	p.dbClient.Close()

	return nil
}

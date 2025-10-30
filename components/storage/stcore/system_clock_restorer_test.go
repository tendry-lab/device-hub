/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package stcore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendry-lab/device-hub/components/status"
)

type testClockReader struct {
	timestamp int64
	err       error
}

func (r *testClockReader) ReadTimestamp(_ context.Context) (int64, error) {
	if r.err != nil {
		return -1, r.err
	}

	return r.timestamp, nil
}

func TestSystemClockRestorerRestoreOK(t *testing.T) {
	timestamp := int64(10)

	reader := &testClockReader{
		timestamp: timestamp,
	}

	restorer := NewSystemClockRestorer(context.Background(), reader)

	ts, err := restorer.GetTimestamp()
	require.Equal(t, int64(-1), ts)
	require.Equal(t, status.StatusInvalidState, err)

	require.Nil(t, restorer.Run())

	ts, err = restorer.GetTimestamp()
	require.Equal(t, timestamp, ts)
	require.Nil(t, err)
}

func TestSystemClockRestorerRestoreNoData(t *testing.T) {
	timestamp := int64(10)

	reader := &testClockReader{
		timestamp: timestamp,
		err:       status.StatusNoData,
	}

	restorer := NewSystemClockRestorer(context.Background(), reader)

	ts, err := restorer.GetTimestamp()
	require.Equal(t, int64(-1), ts)
	require.Equal(t, status.StatusInvalidState, err)

	require.Nil(t, restorer.Run())

	ts, err = restorer.GetTimestamp()
	require.Equal(t, int64(-1), ts)
	require.Nil(t, err)
}

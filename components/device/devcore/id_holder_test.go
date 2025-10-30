/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package devcore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIDHolderGet(t *testing.T) {
	holder := NewIDHolder()
	require.Empty(t, holder.Get())

	deviceID := "0xABCD"

	holder.Set(deviceID)
	require.Equal(t, deviceID, holder.Get())
}

func TestIDHolderGetIDChanged(t *testing.T) {
	holder := NewIDHolder()

	deviceID := "0xABCD"

	holder.Set(deviceID)
	require.Equal(t, deviceID, holder.Get())

	newDeviceID := "0XDCBA"
	require.NotEqual(t, deviceID, newDeviceID)

	holder.Set(newDeviceID)
	require.Equal(t, newDeviceID, holder.Get())
}

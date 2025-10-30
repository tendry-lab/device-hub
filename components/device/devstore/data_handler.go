/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package devstore

import (
	"github.com/tendry-lab/device-hub/components/device/devcore"
	"github.com/tendry-lab/device-hub/components/system/syscore"
)

type dataHandler struct {
	clock   syscore.SystemClock
	builder DataHandlerBuilder
	handler devcore.DataHandler
}

func newDataHandler(clock syscore.SystemClock, builder DataHandlerBuilder) *dataHandler {
	return &dataHandler{
		clock:   clock,
		builder: builder,
	}
}

func (h *dataHandler) HandleTelemetry(deviceID string, js devcore.JSON) error {
	h.buildHanlder(deviceID)
	return h.handler.HandleTelemetry(deviceID, js)
}

func (h *dataHandler) HandleRegistration(deviceID string, js devcore.JSON) error {
	h.buildHanlder(deviceID)
	return h.handler.HandleRegistration(deviceID, js)
}

func (h *dataHandler) buildHanlder(deviceID string) {
	if h.handler == nil {
		h.handler = h.builder.BuildHandler(h.clock, deviceID)
		if h.handler == nil {
			panic("invalid state: handler can't be nil")
		}
	}
}

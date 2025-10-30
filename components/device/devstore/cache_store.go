/*
 * SPDX-FileCopyrightText: 2025 Tendry Lab
 * SPDX-License-Identifier: Apache-2.0
 */

package devstore

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tendry-lab/device-hub/components/device/devcore"
	"github.com/tendry-lab/device-hub/components/http/htcore"
	"github.com/tendry-lab/device-hub/components/status"
	"github.com/tendry-lab/device-hub/components/storage/stcore"
	"github.com/tendry-lab/device-hub/components/system/syscore"
	"github.com/tendry-lab/device-hub/components/system/sysnet"
	"github.com/tendry-lab/device-hub/components/system/syssched"
)

// CacheStoreParams represents various configuration options for a cache store.
type CacheStoreParams struct {
	HTTP struct {
		// FetchInterval - how often to fetch data from the device.
		FetchInterval time.Duration

		// FetchTimeout - how long to wait for the response from the device.
		FetchTimeout time.Duration
	}

	TimeSync struct {
		// Disable to disable automatic device time synchronization.
		//
		// Remarks:
		//  - It doesn't mean that the device timestamp won't be checked.
		Disable bool

		// MaxDriftInterval is a maximum allowed time difference between local
		// and device UNIX time.
		MaxDriftInterval time.Duration

		// How often to perform the timestamp restoring procedure.
		RestoreInterval time.Duration
	}
}

// CacheStore allows to cache information about the added devices in the persistent storage.
type CacheStore struct {
	ctx            context.Context
	localClock     syscore.SystemClock
	readerBuilder  SystemClockReaderBuilder
	handlerBuilder DataHandlerBuilder
	resolveStore   *sysnet.ResolveStore
	aliveMonitor   AliveMonitor
	params         CacheStoreParams

	mu    sync.Mutex
	db    stcore.DB
	nodes map[string]*storeNode
}

// NewCacheStore is an initialization of CacheStore.
//
// Parameters:
//   - ctx - parent context.
//   - localClock to handle local UNIX time.
//   - readerBuilder to build reader for latest device UNIX timestamp.
//   - handlerBuilder to build handler to persist device data.
//   - db to persist device registration life-cycle.
//   - resolveStore to manage device host resolving.
//   - params - various configuration options for a cache store.
func NewCacheStore(
	ctx context.Context,
	localClock syscore.SystemClock,
	readerBuilder SystemClockReaderBuilder,
	handlerBuilder DataHandlerBuilder,
	db stcore.DB,
	resolveStore *sysnet.ResolveStore,
	params CacheStoreParams,
) *CacheStore {
	s := &CacheStore{
		ctx:            ctx,
		localClock:     localClock,
		readerBuilder:  readerBuilder,
		handlerBuilder: handlerBuilder,
		params:         params,
		db:             db,
		resolveStore:   resolveStore,
		nodes:          make(map[string]*storeNode),
	}

	s.restoreNodes()

	return s
}

// SetAliveMonitor sets the device inactivity monitor.
func (s *CacheStore) SetAliveMonitor(monitor AliveMonitor) {
	s.aliveMonitor = monitor
}

// Start starts data processing for cached devices.
func (s *CacheStore) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, node := range s.nodes {
		if err := node.start(); err != nil {
			return err
		}
	}

	return nil
}

// Stop stops data processing for added devices.
func (s *CacheStore) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, node := range s.nodes {
		if err := node.stop(); err != nil {
			syscore.LogErr.Printf("failed to stop device: uri=%s err=%v", node.uri, err)
		}
	}

	s.nodes = nil

	return nil
}

// Add caches the device information in the persistent storage.
func (s *CacheStore) Add(uri string, typ string, desc string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.nodes[uri]; ok {
		return ErrDeviceExist
	}

	now := time.Now()

	node, err := s.makeNode(uri, typ, desc, now)
	if err != nil {
		return err
	}

	item := StorageItem{
		Desc:      desc,
		Timestamp: now.Unix(),
		Type:      typ,
	}

	buf, err := item.MarshalBinary()
	if err != nil {
		return err
	}

	if err := s.db.Write(uri, buf); err != nil {
		return fmt.Errorf("failed to persist device information: uri=%s err=%v", uri, err)
	}

	if err := node.start(); err != nil {
		return err
	}

	s.nodes[uri] = node

	syscore.LogInf.Printf("device added: uri=%s type=%s desc=%s", uri, typ, desc)

	return nil
}

// Remove removes the device if it exists.
func (s *CacheStore) Remove(uri string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[uri]
	if !ok {
		return status.StatusNoData
	}

	if err := s.db.Remove(uri); err != nil {
		return err
	}

	if err := node.stop(); err != nil {
		return fmt.Errorf("failed to stop device: uri=%s err=%v", uri, err)
	}

	delete(s.nodes, uri)

	syscore.LogInf.Printf("device removed: uri=%s", uri)

	return nil
}

// GetDesc returns descriptions for registered devices.
func (s *CacheStore) GetDesc() []StoreItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	var items []StoreItem

	for _, node := range s.nodes {
		items = append(items, StoreItem{
			URI:       node.uri,
			Type:      node.typ,
			Desc:      node.desc,
			ID:        node.holder.Get(),
			CreatedAt: node.createdAt,
		})
	}

	return items
}

func (s *CacheStore) restoreNodes() {
	var unrestoredURIs []string

	err := s.db.ForEach(func(uri string, buf []byte) error {
		if err := s.restoreNode(uri, buf); err != nil {
			syscore.LogErr.Printf("failed to restore device: uri=%s err=%v", uri, err)

			unrestoredURIs = append(unrestoredURIs, uri)
		}

		return nil
	})
	if err != nil {
		panic("failed to restore nodes: invalid state: " + err.Error())
	}

	if len(unrestoredURIs) == 0 {
		return
	}

	for _, uri := range unrestoredURIs {
		if err := s.db.Remove(uri); err != nil {
			syscore.LogErr.Printf("failed to remove unrestored device:"+
				" uri=%s err=%v", uri, err)
		} else {
			syscore.LogErr.Printf("unrestored device removed: uri=%s", uri)
		}
	}
}

func (s *CacheStore) restoreNode(uri string, buf []byte) error {
	var item StorageItem
	if _, err := item.Unmarshal(buf); err != nil {
		return err
	}

	node, err := s.makeNode(uri, item.Type, item.Desc, time.Unix(item.Timestamp, 0))
	if err != nil {
		return err
	}

	s.nodes[uri] = node

	syscore.LogInf.Printf("device restored: uri=%s type=%s desc=%s", uri, item.Type, item.Desc)

	return nil
}

func (s *CacheStore) makeNode(
	uri string,
	typ string,
	desc string,
	now time.Time,
) (*storeNode, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	deviceType := parseDeviceType(u.Scheme)

	switch deviceType {
	case deviceTypeHTTP:
		return s.makeNodeHTTP(u, uri, typ, desc, now)
	default:
		return nil, status.StatusNotSupported
	}
}

func (s *CacheStore) makeNodeHTTP(
	u *url.URL,
	uri string,
	typ string,
	desc string,
	now time.Time,
) (*storeNode, error) {
	if u.Port() == "" {
		return nil, fmt.Errorf("HTTP port is missed")
	}

	ctx, cancelFunc := context.WithCancel(s.ctx)

	stopper := &syssched.FanoutStopper{}
	starter := &syssched.FanoutStarter{}

	idHolder := devcore.NewIDHolder()

	clockReader := newSystemClockReader(idHolder, s.readerBuilder)
	clockRestorer := stcore.NewSystemClockRestorer(ctx, clockReader)

	clockRestorerRunner := syssched.NewAsyncTaskRunner(
		ctx,
		clockRestorer,
		clockRestorer,
		syssched.AsyncTaskRunnerParams{
			UpdateInterval: s.params.TimeSync.RestoreInterval,
			ExitOnSuccess:  true,
		},
	)

	starter.Add(clockRestorerRunner)
	stopper.Add(uri+"-clock-restorer", clockRestorerRunner)

	deviceRunner := syssched.NewAsyncTaskRunner(
		ctx,
		s.newHTTPDevice(
			ctx,
			stopper,
			idHolder,
			newDataHandler(clockRestorer, s.handlerBuilder),
			s.localClock,
			clockRestorer,
			uri,
			desc,
			u.Hostname(),
		),
		&logErrorHandler{uri: uri, typ: typ, desc: desc},
		syssched.AsyncTaskRunnerParams{
			UpdateInterval: s.params.HTTP.FetchInterval,
		},
	)

	starter.Add(deviceRunner)
	stopper.Add(uri+"-device-http", deviceRunner)

	return &storeNode{
		uri:        uri,
		typ:        typ,
		desc:       desc,
		createdAt:  now.Format(time.RFC1123),
		holder:     idHolder,
		cancelFunc: cancelFunc,
		stopper:    stopper,
		starter:    starter,
	}, nil
}

func (s *CacheStore) newHTTPDevice(
	ctx context.Context,
	stopper *syssched.FanoutStopper,
	idHolder *devcore.IDHolder,
	dataHandler devcore.DataHandler,
	localClock syscore.SystemClock,
	remoteLastClock syscore.SystemClock,
	uri string,
	desc string,
	hostname string,
) syssched.Task {
	var clockSynchronizer devcore.TimeSynchronizer
	if s.params.TimeSync.Disable {
		clockSynchronizer = devcore.FuncSynchronizer(func() error {
			return status.StatusNotSupported
		})
	} else {
		remoteCurrClock := htcore.NewSystemClock(
			ctx,
			s.makeHTTPClient(stopper, uri, desc, hostname),
			uri+"/system/time",
			s.params.HTTP.FetchTimeout,
		)

		clockSynchronizer = syscore.NewSystemClockSynchronizer(
			localClock, remoteLastClock, remoteCurrClock)
	}

	var clockVerifier devcore.TimeVerifier
	if maxDriftInterval := s.params.TimeSync.MaxDriftInterval; maxDriftInterval == 0 {
		clockVerifier = &devcore.BasicTimeVerifier{}
	} else {
		clockVerifier = devcore.NewDriftTimeVerifier(localClock, maxDriftInterval)
	}

	task := devcore.NewPollDevice(
		htcore.NewURLFetcher(
			ctx,
			s.makeHTTPClient(stopper, uri, desc, hostname),
			uri+"/registration",
			s.params.HTTP.FetchTimeout,
		),
		htcore.NewURLFetcher(
			ctx,
			s.makeHTTPClient(stopper, uri, desc, hostname),
			uri+"/telemetry",
			s.params.HTTP.FetchTimeout,
		),
		idHolder,
		dataHandler,
		clockSynchronizer,
		clockVerifier,
	)

	if s.aliveMonitor != nil {
		notifier := s.aliveMonitor.Monitor(uri)

		return syssched.NewTaskAliveNotifier(task, notifier)
	}

	return task
}

func (s *CacheStore) makeHTTPClient(
	stopper *syssched.FanoutStopper,
	uri string,
	desc string,
	hostname string,
) *htcore.HTTPClient {
	if !strings.Contains(uri, ".local") {
		return htcore.NewDefaultClient()
	}

	s.resolveStore.Add(hostname)

	stopper.Add("resolve-store-"+desc, syssched.FuncStopper(func() error {
		s.resolveStore.Remove(hostname)

		return nil
	}))

	return htcore.NewResolveClient(s.resolveStore)
}

type deviceType int

const (
	deviceTypeUnsupported deviceType = iota
	deviceTypeHTTP
)

func parseDeviceType(scheme string) deviceType {
	if scheme == "http" || scheme == "https" {
		return deviceTypeHTTP
	}

	return deviceTypeUnsupported
}

type storeNode struct {
	uri        string
	typ        string
	desc       string
	createdAt  string
	holder     *devcore.IDHolder
	cancelFunc context.CancelFunc
	stopper    *syssched.FanoutStopper
	starter    *syssched.FanoutStarter
}

func (s *storeNode) start() error {
	return s.starter.Start()
}

func (s *storeNode) stop() error {
	s.cancelFunc()

	return s.stopper.Stop()
}

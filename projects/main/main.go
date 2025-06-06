package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.etcd.io/bbolt"

	"github.com/tendry-lab/zeroconf"

	"github.com/tendry-lab/device-hub/components/device/devstore"
	"github.com/tendry-lab/device-hub/components/http/htcore"
	"github.com/tendry-lab/device-hub/components/http/hthandler"
	"github.com/tendry-lab/device-hub/components/storage/stcore"
	"github.com/tendry-lab/device-hub/components/storage/stinfluxdb"
	"github.com/tendry-lab/device-hub/components/system/syscore"
	"github.com/tendry-lab/device-hub/components/system/sysmdns"
	"github.com/tendry-lab/device-hub/components/system/sysnet"
	"github.com/tendry-lab/device-hub/components/system/syssched"
)

type appOptions struct {
	logDir   string
	cacheDir string
	port     int

	storage struct {
		influxdb stinfluxdb.DBParams
	}

	device struct {
		http struct {
			fetchTimeout  string
			fetchInterval string
		}

		monitor struct {
			inactive struct {
				disable        bool
				maxInterval    string
				updateInterval string
			}
		}

		timeSync struct {
			disable          bool
			maxDriftInterval string
		}
	}

	mdns struct {
		browse struct {
			interval string
			timeout  string
			iface    string
		}

		autodiscovery struct {
			disable bool
		}

		server struct {
			disable  bool
			hostname string
			iface    string
		}
	}
}

type appPipeline struct {
	stopper     *syssched.FanoutStopper
	starter     *syssched.FanoutStarter
	systemClock syscore.SystemClock
}

func (p *appPipeline) start(opts *appOptions) error {
	appContext, cancelFunc := signal.NotifyContext(context.Background(),
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	defer cancelFunc()

	resolveStore := sysnet.NewResolveStore()
	resolveServiceHandler := sysmdns.NewResolveServiceHandler(resolveStore)

	fanoutServiceHandler := &sysmdns.FanoutServiceHandler{}
	fanoutServiceHandler.Add(resolveServiceHandler)

	mdnsBrowseAwakener, err := p.createMdnsBrowser(appContext, fanoutServiceHandler, opts)
	if err != nil {
		return err
	}

	deviceStore, err := p.createDeviceStore(appContext, resolveStore, mdnsBrowseAwakener, opts)
	if err != nil {
		return err
	}

	if !opts.mdns.autodiscovery.disable {
		storeMdnsHandler := devstore.NewStoreMdnsHandler(deviceStore)
		fanoutServiceHandler.Add(storeMdnsHandler)
	}

	mux := http.NewServeMux()
	crashHandler := hthandler.NewCrashHandler(mux)

	server, err := htcore.NewServer(crashHandler, htcore.ServerParams{
		Port: opts.port,
	})
	if err != nil {
		return err
	}
	p.stopper.Add("http-server", server)
	p.starter.Add(server)

	registerHTTPRoutes(
		mux,
		// Time valid since 2024/12/03.
		hthandler.NewSystemTimeHandler(p.systemClock, time.Unix(1733215816, 0)),
		devstore.NewStoreHTTPHandler(deviceStore),
	)

	if !opts.mdns.server.disable {
		if err := p.configureMdnsServer(server, opts); err != nil {
			return err
		}
	}

	if err := p.starter.Start(); err != nil {
		return err
	}

	<-appContext.Done()

	return nil
}

func (p *appPipeline) stop() error {
	return p.stopper.Stop()
}

func (p *appPipeline) createDeviceStore(
	ctx context.Context,
	resolveStore *sysnet.ResolveStore,
	awakener syssched.Awakener,
	opts *appOptions,
) (devstore.Store, error) {
	cacheStore, err := p.createCacheStore(ctx, resolveStore, opts)
	if err != nil {
		return nil, err
	}

	awakeStore := devstore.NewAwakeStore(awakener, cacheStore)

	if opts.device.monitor.inactive.disable {
		return awakeStore, nil
	}

	inactiveMaxInterval, err :=
		time.ParseDuration(opts.device.monitor.inactive.maxInterval)
	if err != nil {
		return nil, err
	}

	if inactiveMaxInterval < time.Millisecond {
		return nil, errors.New("device-monitor-inactive-max-interval can't be" +
			" less than 1ms")
	}

	inactiveUpdateInterval, err :=
		time.ParseDuration(opts.device.monitor.inactive.updateInterval)
	if err != nil {
		return nil, err
	}

	if inactiveUpdateInterval < time.Millisecond {
		return nil, errors.New("device-monitor-inactive-update-interval can't be" +
			" less than 1ms")
	}

	aliveMonitor := devstore.NewStoreAliveMonitor(
		&syscore.LocalMonotonicClock{},
		awakeStore,
		inactiveMaxInterval,
	)
	cacheStore.SetAliveMonitor(aliveMonitor)

	aliveMonitorRunner := syssched.NewAsyncTaskRunner(
		ctx,
		aliveMonitor,
		aliveMonitor,
		syssched.AsyncTaskRunnerParams{
			UpdateInterval: inactiveUpdateInterval,
		},
	)

	p.stopper.Add("device-alive-monitor-runner", aliveMonitorRunner)
	p.starter.Add(aliveMonitorRunner)

	return aliveMonitor, nil
}

func (p *appPipeline) createMdnsBrowser(
	ctx context.Context,
	fanoutServiceHandler *sysmdns.FanoutServiceHandler,
	opts *appOptions,
) (syssched.Awakener, error) {
	mdnsBrowseInterval, err := time.ParseDuration(opts.mdns.browse.interval)
	if err != nil {
		return nil, err
	}
	if mdnsBrowseInterval < time.Second {
		return nil, errors.New("mDNS browse interval can't be less than 1s")
	}

	mdnsBrowseTimeout, err := time.ParseDuration(opts.mdns.browse.timeout)
	if err != nil {
		return nil, err
	}
	if mdnsBrowseTimeout < time.Second {
		return nil, errors.New("mDNS browse timeout can't be less than 1s")
	}

	filteredIfaces, err := parseIfaceOption(opts.mdns.browse.iface)
	if err != nil {
		return nil, err
	}

	mdnsBrowser := sysmdns.NewZeroconfBrowser(
		ctx,
		fanoutServiceHandler,
		sysmdns.ZeroconfBrowserParams{
			Service: sysmdns.ServiceName(sysmdns.ServiceTypeHTTP, sysmdns.ProtoTCP),
			Domain:  "local",
			Timeout: mdnsBrowseTimeout,
			Opts: []zeroconf.ClientOption{
				zeroconf.SelectIfaces(filteredIfaces),
			},
		},
	)
	p.stopper.Add("mdns-zeroconf-browser", mdnsBrowser)

	mdnsBrowserRunner := syssched.NewAsyncTaskRunner(
		ctx,
		mdnsBrowser,
		mdnsBrowser,
		syssched.AsyncTaskRunnerParams{
			UpdateInterval: mdnsBrowseInterval,
		},
	)
	p.stopper.Add("mdns-zeroconf-browser-runner", mdnsBrowserRunner)
	p.starter.Add(mdnsBrowserRunner)

	return mdnsBrowserRunner, nil
}

func (p *appPipeline) createCacheStore(
	ctx context.Context,
	resolveStore *sysnet.ResolveStore,
	opts *appOptions,
) (*devstore.CacheStore, error) {
	fetchInterval, err := time.ParseDuration(opts.device.http.fetchInterval)
	if err != nil {
		return nil, err
	}
	if fetchInterval < time.Millisecond {
		return nil, errors.New("HTTP device fetch interval can't be less than 1ms")
	}

	fetchTimeout, err := time.ParseDuration(opts.device.http.fetchTimeout)
	if err != nil {
		return nil, err
	}
	if fetchTimeout < time.Millisecond {
		return nil, errors.New("HTTP device fetch timeout can't be less than 1ms")
	}

	var maxDriftInterval time.Duration

	if opts.device.timeSync.maxDriftInterval != "" {
		interval, err := time.ParseDuration(opts.device.timeSync.maxDriftInterval)
		if err != nil {
			return nil, err
		}
		if interval < time.Second {
			return nil, errors.New("--device-time-sync-drift-interval can't be less than 1s")
		}

		maxDriftInterval = interval
	}

	cacheStoreParams := devstore.CacheStoreParams{}
	cacheStoreParams.HTTP.FetchInterval = fetchInterval
	cacheStoreParams.HTTP.FetchTimeout = fetchTimeout
	cacheStoreParams.TimeSync.MaxDriftInterval = maxDriftInterval
	cacheStoreParams.TimeSync.Disable = opts.device.timeSync.disable

	db, err := p.createDB(opts)
	if err != nil {
		return nil, err
	}

	storagePipeline := stinfluxdb.NewPipeline(ctx, opts.storage.influxdb)
	p.stopper.Add("storage-influxdb-pipeline", storagePipeline)
	p.starter.Add(storagePipeline)

	cacheStore := devstore.NewCacheStore(
		ctx,
		p.systemClock,
		storagePipeline.GetSystemClock(),
		storagePipeline.GetDataHandler(),
		db,
		resolveStore,
		cacheStoreParams,
	)
	p.stopper.Add("device-cache-store", cacheStore)
	p.starter.Add(cacheStore)

	return cacheStore, nil
}

func (p *appPipeline) createDB(opts *appOptions) (stcore.DB, error) {
	if opts.cacheDir == "" {
		return &stcore.NoopDB{}, nil
	}

	opts.cacheDir = path.Join(opts.cacheDir, "bbolt.db")

	bboltDB, err := stcore.NewBboltDB(opts.cacheDir, &bbolt.Options{
		Timeout: time.Second * 5,
	})
	if err != nil {
		return nil, err
	}

	p.stopper.Add("bbolt-database", syssched.FuncStopper(func() error {
		return bboltDB.Close()
	}))

	return stcore.NewBboltDBBucket(bboltDB, "device_bucket"), nil
}

func (p *appPipeline) configureMdnsServer(server *htcore.Server, opts *appOptions) error {
	services := []*sysmdns.Service{
		{
			Instance:   "Device Hub HTTP Service",
			Name:       sysmdns.ServiceName(sysmdns.ServiceTypeHTTP, sysmdns.ProtoTCP),
			Hostname:   opts.mdns.server.hostname,
			Port:       server.Port(),
			TxtRecords: []string{"api=/api/v1"},
		},
	}

	filteredIfaces, err := parseIfaceOption(opts.mdns.server.iface)
	if err != nil {
		return err
	}

	zeroconfServer := sysmdns.NewZeroconfServer(services, filteredIfaces)
	p.stopper.Add("mdns-server", zeroconfServer)
	p.starter.Add(zeroconfServer)

	return nil
}

func parseIfaceOption(opt string) ([]net.Interface, error) {
	var allowedIfaces []string

	if opt != "" {
		allowedIfaces = strings.Split(opt, ",")
		if len(allowedIfaces) < 1 {
			return nil, errors.New("mDNS network interface list has invalid format")
		}
	}

	filteredIfaces, err := sysnet.FilterInterfaces(func(iface net.Interface) bool {
		if iface.Flags&net.FlagMulticast == 0 {
			return false
		}

		if allowedIfaces == nil {
			return true
		}

		for _, allowedIface := range allowedIfaces {
			if allowedIface == iface.Name {
				return true
			}
		}

		return false
	})
	if err != nil {
		return nil, err
	}

	return filteredIfaces, nil
}

func registerHTTPRoutes(
	mux *http.ServeMux,
	timeHandler http.Handler,
	storeHTTPHandler *devstore.StoreHTTPHandler,
) {
	mux.Handle("/api/v1/system/time", timeHandler)

	mux.HandleFunc("/api/v1/device/add", storeHTTPHandler.HandleAdd)
	mux.HandleFunc("/api/v1/device/remove", storeHTTPHandler.HandleRemove)
	mux.HandleFunc("/api/v1/device/list", storeHTTPHandler.HandleList)
}

func newAppPipeline() *appPipeline {
	return &appPipeline{
		systemClock: &syscore.LocalSystemClock{},
		stopper:     &syssched.FanoutStopper{},
		starter:     &syssched.FanoutStarter{},
	}
}

func prepareEnvironment(opts *appOptions) error {
	if opts.storage.influxdb.URL == "" {
		return fmt.Errorf("influxdb URL is required")
	}
	if opts.storage.influxdb.Org == "" {
		return fmt.Errorf("influxdb org is required")
	}
	if opts.storage.influxdb.Bucket == "" {
		return fmt.Errorf("influxdb bucket is required")
	}
	if opts.storage.influxdb.Token == "" {
		return fmt.Errorf("influxdb token is required")
	}

	if opts.cacheDir != "" {
		fi, err := os.Stat(opts.cacheDir)
		if err != nil {
			return err
		}

		if !fi.Mode().IsDir() {
			return errors.New("cache path should be a directory")
		}
	}

	if opts.logDir == "" {
		return fmt.Errorf("log directory is required")
	}
	fi, err := os.Stat(opts.logDir)
	if err != nil {
		return err
	}
	if !fi.Mode().IsDir() {
		return errors.New("log path should be a directory")
	}
	if err := syscore.SetLogFile(filepath.Join(opts.logDir, "app.log")); err != nil {
		return err
	}

	if !opts.mdns.server.disable {
		if opts.mdns.server.hostname == "" {
			return errors.New("mDNS server hostname can't be empty")
		}
	}

	return nil
}

func main() {
	pipeline := newAppPipeline()
	options := &appOptions{}

	cmd := &cobra.Command{
		Use:           "device-hub",
		Short:         "device-hub CLI",
		Long:          "device-hub collects and stores various data from IoT devices",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return prepareEnvironment(options)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := pipeline.start(options); err != nil {
				return err
			}

			return pipeline.stop()
		},
	}

	cmd.Flags().IntVar(&options.port, "http-port", 0,
		"HTTP server port (0 for random port)")

	cmd.Flags().StringVar(&options.cacheDir, "cache-dir", "", "cache directory")
	cmd.Flags().StringVar(&options.logDir, "log-dir", "", "log directory")

	cmd.Flags().StringVar(&options.storage.influxdb.URL, "storage-influxdb-url", "",
		"influxdb URL")
	cmd.Flags().StringVar(&options.storage.influxdb.Org, "storage-influxdb-org", "",
		"influxdb Org")
	cmd.Flags().StringVar(&options.storage.influxdb.Token, "storage-influxdb-api-token", "",
		"influxdb API token")
	cmd.Flags().StringVar(&options.storage.influxdb.Bucket, "storage-influxdb-bucket", "",
		"influxdb bucket")

	cmd.Flags().StringVar(
		&options.device.http.fetchInterval,
		"device-http-fetch-interval", "5s",
		"HTTP device data fetch interval",
	)
	cmd.Flags().StringVar(
		&options.device.http.fetchTimeout,
		"device-http-fetch-timeout", "5s",
		"HTTP device data fetch timeout",
	)

	cmd.Flags().StringVar(
		&options.device.monitor.inactive.maxInterval,
		"device-monitor-inactive-max-interval", "2m",
		"How long it's allowed for a device to be inactive",
	)

	cmd.Flags().StringVar(
		&options.device.monitor.inactive.updateInterval,
		"device-monitor-inactive-update-interval", "10s",
		"How often to check for a device inactivity",
	)

	cmd.Flags().BoolVar(
		&options.device.monitor.inactive.disable,
		"device-monitor-inactive-disable", false,
		"Disable device inactivity monitoring",
	)

	cmd.Flags().BoolVar(
		&options.device.timeSync.disable,
		"device-time-sync-disable", false,
		"Disable automatic device time synchronization",
	)

	cmd.Flags().StringVar(
		&options.device.timeSync.maxDriftInterval,
		"device-time-sync-drift-interval", "5s",
		"Maximum allowed time drift between local and device UNIX time"+
			" (empty to disable drift check)",
	)

	cmd.Flags().StringVar(
		&options.mdns.browse.interval,
		"mdns-browse-interval", "40s",
		"How often to perform mDNS lookup over local network",
	)

	cmd.Flags().StringVar(
		&options.mdns.browse.timeout,
		"mdns-browse-timeout", "10s",
		"How long to perform a single mDNS lookup over local network",
	)

	cmd.Flags().StringVar(
		&options.mdns.browse.iface,
		"mdns-browse-iface", "",
		"Comma-separated list of network interfaces for the mDNS lookup"+
			" (empty for all interfaces)",
	)

	cmd.Flags().BoolVar(
		&options.mdns.autodiscovery.disable,
		"mdns-autodiscovery-disable", false,
		"Disable automatic device discovery on the local network",
	)

	cmd.Flags().BoolVar(
		&options.mdns.server.disable,
		"mdns-server-disable", false,
		"Disable mDNS server",
	)

	cmd.Flags().StringVar(
		&options.mdns.server.hostname,
		"mdns-server-hostname", "device-hub",
		"mDNS server hostname",
	)

	cmd.Flags().StringVar(
		&options.mdns.server.iface,
		"mdns-server-iface", "",
		"Comma-separated list of network interfaces for the mDNS server"+
			" (empty for all interfaces)",
	)

	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: failed to execute command: %v", err)
		os.Exit(1)
	}
}

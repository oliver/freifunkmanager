package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/genofire/golang-lib/file"
	httpLib "github.com/genofire/golang-lib/http"
	"github.com/genofire/golang-lib/worker"
	log "github.com/sirupsen/logrus"

	respondYanic "github.com/FreifunkBremen/yanic/respond"
	runtimeYanic "github.com/FreifunkBremen/yanic/runtime"

	"github.com/FreifunkBremen/freifunkmanager/runtime"
	"github.com/FreifunkBremen/freifunkmanager/ssh"
	"github.com/FreifunkBremen/freifunkmanager/websocket"
)

var (
	configFile string
	config     = &runtime.Config{}
	nodes      *runtime.Nodes
	collector  *respondYanic.Collector
	verbose    bool
)

func main() {
	flag.StringVar(&configFile, "config", "config.conf", "path of configuration file (default:config.conf)")
	flag.BoolVar(&verbose, "v", false, "verbose logging")
	flag.Parse()
	if verbose {
		log.SetLevel(log.DebugLevel)
	}

	if err := file.ReadTOML(configFile, config); err != nil {
		log.Panicf("Error during read config: %s", err)
	}

	log.Info("starting...")

	sshmanager := ssh.NewManager(config.SSHPrivateKey, config.SSHTimeout.Duration)
	nodes = runtime.NewNodes(config.StatePath, config.SSHInterface, sshmanager)
	nodesSaveWorker := worker.NewWorker(time.Duration(3)*time.Second, nodes.Saver)
	nodesUpdateWorker := worker.NewWorker(time.Duration(3)*time.Minute, nodes.Updater)
	nodesYanic := runtimeYanic.NewNodes(&runtimeYanic.NodesConfig{})

	db := runtime.NewYanicDB(nodes, config.SSHIPAddressSuffix)
	go nodesSaveWorker.Start()
	go nodesUpdateWorker.Start()

	ws := websocket.NewWebsocketServer(config.Secret, nodes)
	nodes.AddNotifyStats(ws.SendStats)
	nodes.AddNotifyNode(ws.SendNode)

	if config.YanicEnable {
		if duration := config.YanicSynchronize.Duration; duration > 0 {
			now := time.Now()
			delay := duration - now.Sub(now.Truncate(duration))
			log.Printf("delaying %0.1f seconds", delay.Seconds())
			time.Sleep(delay)
		}
		collector = respondYanic.NewCollector(db, nodesYanic, make(map[string][]string), []respondYanic.InterfaceConfig{config.Yanic})
		if duration := config.YanicCollectInterval.Duration; duration > 0 {
			collector.Start(config.YanicCollectInterval.Duration)
		}
		defer collector.Close()
		log.Info("started Yanic collector")
	}

	// Startwebserver
	http.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		httpLib.Write(w, nodes)
	})
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		httpLib.Write(w, nodes.Statistics)
	})
	http.Handle("/", gziphandler.GzipHandler(http.FileServer(http.Dir(config.Webroot))))

	srv := &http.Server{
		Addr: config.WebserverBind,
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Panic(err)
		}
	}()

	log.Info("started")

	// Wait for system signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs

	ws.Close()

	// Stop services
	srv.Close()
	nodesSaveWorker.Close()
	nodesUpdateWorker.Close()

	log.Info("stop recieve:", sig)
}

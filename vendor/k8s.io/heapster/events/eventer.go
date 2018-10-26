// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:generate ./hooks/run_extpoints.sh

package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/apiserver/pkg/util/logs"
	"k8s.io/heapster/common/flags"
	"k8s.io/heapster/events/api"
	"k8s.io/heapster/events/manager"
	"k8s.io/heapster/events/sinks"
	"k8s.io/heapster/events/sources"
	"k8s.io/heapster/version"
)

var (
	argFrequency   = flag.Duration("frequency", 30*time.Second, "The resolution at which Eventer pushes events to sinks")
	argMaxProcs    = flag.Int("max_procs", 0, "max number of CPUs that can be used simultaneously. Less than 1 for default (number of cores)")
	argSources     flags.Uris
	argSinks       flags.Uris
	argVersion     bool
	argHealthzIP   = flag.String("healthz-ip", "0.0.0.0", "ip eventer health check service uses")
	argHealthzPort = flag.Uint("healthz-port", 8084, "port eventer health check listens on")
)

func main() {
	quitChannel := make(chan struct{}, 0)

	flag.Var(&argSources, "source", "source(s) to read events from")
	flag.Var(&argSinks, "sink", "external sink(s) that receive events")
	flag.BoolVar(&argVersion, "version", false, "print version info and exit")
	flag.Parse()

	if argVersion {
		fmt.Println(version.VersionInfo())
		os.Exit(0)
	}

	logs.InitLogs()
	defer logs.FlushLogs()

	setMaxProcs()

	glog.Infof(strings.Join(os.Args, " "))
	glog.Infof("Eventer version %v", version.HeapsterVersion)
	if err := validateFlags(); err != nil {
		glog.Fatal(err)
	}

	// sources
	if len(argSources) != 1 {
		glog.Fatal("Wrong number of sources specified")
	}
	sourceFactory := sources.NewSourceFactory()
	sources, err := sourceFactory.BuildAll(argSources)
	if err != nil {
		glog.Fatalf("Failed to create sources: %v", err)
	}
	if len(sources) != 1 {
		glog.Fatal("Requires exactly 1 source")
	}

	// sinks
	sinksFactory := sinks.NewSinkFactory()
	sinkList := sinksFactory.BuildAll(argSinks)
	if len([]flags.Uri(argSinks)) != 0 && len(sinkList) == 0 {
		glog.Fatal("No available sink to use")
	}

	for _, sink := range sinkList {
		glog.Infof("Starting with %s sink", sink.Name())
	}
	sinkManager, err := sinks.NewEventSinkManager(sinkList, sinks.DefaultSinkExportEventsTimeout, sinks.DefaultSinkStopTimeout)
	if err != nil {
		glog.Fatalf("Failed to create sink manager: %v", err)
	}

	// main manager
	manager, err := manager.NewManager(sources[0], sinkManager, *argFrequency)
	if err != nil {
		glog.Fatalf("Failed to create main manager: %v", err)
	}

	manager.Start()
	glog.Infof("Starting eventer")

	go startHTTPServer()

	<-quitChannel
}

func startHTTPServer() {
	glog.Info("Starting eventer http service")

	glog.Fatal(http.ListenAndServe(net.JoinHostPort(*argHealthzIP, strconv.Itoa(int(*argHealthzPort))), nil))
}

func validateFlags() error {
	var minFrequency = 5 * time.Second

	if *argFrequency < minFrequency {
		return fmt.Errorf("frequency needs to be no less than %s, supplied %s", minFrequency,
			*argFrequency)
	}

	if *argFrequency > api.MaxEventsScrapeDelay {
		return fmt.Errorf("frequency needs to be no greater than %s, supplied %s",
			api.MaxEventsScrapeDelay, *argFrequency)
	}

	return nil
}

func setMaxProcs() {
	// Allow as many threads as we have cores unless the user specified a value.
	var numProcs int
	if *argMaxProcs < 1 {
		numProcs = runtime.NumCPU()
	} else {
		numProcs = *argMaxProcs
	}
	runtime.GOMAXPROCS(numProcs)

	// Check if the setting was successful.
	actualNumProcs := runtime.GOMAXPROCS(0)
	if actualNumProcs != numProcs {
		glog.Warningf("Specified max procs of %d but using %d", numProcs, actualNumProcs)
	}
}

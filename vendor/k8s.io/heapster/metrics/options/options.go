// Copyright 2016 Google Inc. All Rights Reserved.
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

package options

import (
	"time"

	"github.com/spf13/pflag"

	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/heapster/common/flags"
)

type HeapsterRunOptions struct {
	// genericoptions.ReccomendedOptions - EtcdOptions
	SecureServing  *genericoptions.SecureServingOptions
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions
	Features       *genericoptions.FeatureOptions

	// Only to be used to for testing
	DisableAuthForTesting bool

	MetricResolution      time.Duration
	EnableAPIServer       bool
	Port                  int
	Ip                    string
	MaxProcs              int
	TLSCertFile           string
	TLSKeyFile            string
	TLSClientCAFile       string
	AllowedUsers          string
	Sources               flags.Uris
	Sinks                 flags.Uris
	HistoricalSource      string
	Version               bool
	LabelSeparator        string
	IgnoredLabels         []string
	StoredLabels          []string
	DisableMetricExport   bool
	SinkExportDataTimeout time.Duration
	DisableMetricSink     bool
}

func NewHeapsterRunOptions() *HeapsterRunOptions {
	return &HeapsterRunOptions{
		SecureServing:  genericoptions.NewSecureServingOptions(),
		Authentication: genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:  genericoptions.NewDelegatingAuthorizationOptions(),
		Features:       genericoptions.NewFeatureOptions(),
	}
}

func (h *HeapsterRunOptions) AddFlags(fs *pflag.FlagSet) {
	h.SecureServing.AddFlags(fs)
	h.Authentication.AddFlags(fs)
	h.Authorization.AddFlags(fs)
	h.Features.AddFlags(fs)

	fs.Var(&h.Sources, "source", "source(s) to watch")
	fs.Var(&h.Sinks, "sink", "external sink(s) that receive data")
	fs.DurationVar(&h.MetricResolution, "metric_resolution", 60*time.Second, "The resolution at which heapster will retain metrics.")

	// TODO: Revise these flags before Heapster v1.3 and Kubernetes v1.5
	fs.BoolVar(&h.EnableAPIServer, "api-server", false, "Enable API server for the Metrics API. "+
		"If set, the Metrics API will be served on --insecure-port (internally) and --secure-port (externally).")
	fs.IntVar(&h.Port, "heapster-port", 8082, "port used by the Heapster-specific APIs")

	fs.StringVar(&h.Ip, "listen_ip", "", "IP to listen on, defaults to all IPs")
	fs.IntVar(&h.MaxProcs, "max_procs", 0, "max number of CPUs that can be used simultaneously. Less than 1 for default (number of cores)")
	fs.StringVar(&h.TLSCertFile, "tls_cert", "", "file containing TLS certificate")
	fs.StringVar(&h.TLSKeyFile, "tls_key", "", "file containing TLS key")
	fs.StringVar(&h.TLSClientCAFile, "tls_client_ca", "", "file containing TLS client CA for client cert validation")
	fs.StringVar(&h.AllowedUsers, "allowed_users", "", "comma-separated list of allowed users")
	fs.StringVar(&h.HistoricalSource, "historical_source", "", "which source type to use for the historical API (should be exactly the same as one of the sink URIs), or empty to disable the historical API")
	fs.BoolVar(&h.Version, "version", false, "print version info and exit")
	fs.StringVar(&h.LabelSeparator, "label_separator", ",", "separator used for joining labels")
	fs.StringSliceVar(&h.IgnoredLabels, "ignore_label", []string{}, "ignore this label when joining labels")
	fs.StringSliceVar(&h.StoredLabels, "store_label", []string{}, "store this label separately from joined labels with the same name (name) or with different name (newName=name)")
	fs.BoolVar(&h.DisableMetricExport, "disable_export", false, "Disable exporting metrics in api/v1/metric-export")
	fs.DurationVar(&h.SinkExportDataTimeout, "sink_export_data_timeout", 20*time.Second, "Timeout for exporting data to a sink")
	fs.BoolVar(&h.DisableMetricSink, "disable_metric_sink", false, "Disable metric sink")
}

// Copyright 2015 Google Inc. All Rights Reserved.
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

package processors

import (
	"net/url"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/fields"
	kube_client "k8s.io/client-go/kubernetes"
	kube_api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
	kube_config "k8s.io/heapster/common/kubernetes"
	"k8s.io/heapster/metrics/core"
)

type NamespaceBasedEnricher struct {
	store     cache.Store
	reflector *cache.Reflector
}

func (this *NamespaceBasedEnricher) Name() string {
	return "namespace_based_enricher"
}

func (this *NamespaceBasedEnricher) Process(batch *core.DataBatch) (*core.DataBatch, error) {
	for _, ms := range batch.MetricSets {
		this.addNamespaceInfo(ms)
	}
	return batch, nil
}

// Adds UID to all namespaced elements.
func (this *NamespaceBasedEnricher) addNamespaceInfo(metricSet *core.MetricSet) {
	metricSetType, found := metricSet.Labels[core.LabelMetricSetType.Key]
	if !found {
		return
	}
	if metricSetType != core.MetricSetTypePodContainer &&
		metricSetType != core.MetricSetTypePod &&
		metricSetType != core.MetricSetTypeNamespace {
		return
	}

	namespaceName, found := metricSet.Labels[core.LabelNamespaceName.Key]
	if !found {
		return
	}

	nsObj, exists, err := this.store.GetByKey(namespaceName)
	if exists && err == nil {
		namespace, ok := nsObj.(*kube_api.Namespace)
		if ok {
			metricSet.Labels[core.LabelPodNamespaceUID.Key] = string(namespace.UID)
		} else {
			glog.Errorf("Wrong namespace store content")
		}
	} else if err != nil {
		glog.Warningf("Failed to get namespace %s: %v", namespaceName, err)
	} else if !exists {
		glog.Warningf("Namespace doesn't exist: %s", namespaceName)
	}
}

func NewNamespaceBasedEnricher(url *url.URL) (*NamespaceBasedEnricher, error) {
	kubeConfig, err := kube_config.GetKubeClientConfig(url)
	if err != nil {
		return nil, err
	}
	kubeClient := kube_client.NewForConfigOrDie(kubeConfig)

	// watch nodes
	lw := cache.NewListWatchFromClient(kubeClient.Core().RESTClient(), "namespaces", kube_api.NamespaceAll, fields.Everything())
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	reflector := cache.NewReflector(lw, &kube_api.Namespace{}, store, time.Hour)
	reflector.Run()

	return &NamespaceBasedEnricher{
		store:     store,
		reflector: reflector,
	}, nil
}

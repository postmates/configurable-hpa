/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// copy of ./pkg/controller/configurablehorizontalpodautoscaler/replica_calculator.go
// from kubernetes-1.5.8
//  + some fixes to compile it
package configurablehorizontalpodautoscaler

import (
	"fmt"
	"log"
	"math"
	"time"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	podv1 "k8s.io/kubernetes/pkg/api/v1/pod"
	metricsclient "k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
)

type ReplicaCalculator struct {
	metricsClient metricsclient.MetricsClient
	podsGetter    corev1.PodsGetter
}

func NewReplicaCalculator(metricsClient metricsclient.MetricsClient, podsGetter corev1.PodsGetter) *ReplicaCalculator {
	return &ReplicaCalculator{
		metricsClient: metricsClient,
		podsGetter:    podsGetter,
	}
}

// GetResourceReplicas calculates the desired replica count based on a target resource utilization percentage
// of the given resource for pods matching the given selector in the given namespace, and the current replica count
func (c *ReplicaCalculator) GetResourceReplicas(currentReplicas int32, targetUtilization int32, resource apiv1.ResourceName, namespace string, labelsSelector labels.Selector) (replicaCount int32, utilization int32, timestamp time.Time, err error) {
	selector := labelsSelector.String()
	metrics, timestamp, err := c.metricsClient.GetResourceMetric(resource, namespace, labelsSelector)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("unable to get metrics for resource %s: %v", resource, err)
	}

	log.Printf("metrics: %v\n", metrics)
	podList, err := c.podsGetter.Pods(namespace).List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("unable to get pods while calculating replica count: %v", err)
	}

	if len(podList.Items) == 0 {
		return 0, 0, time.Time{}, fmt.Errorf("no pods returned by selector while calculating replica count")
	}

	requests := make(map[string]int64, len(podList.Items))
	readyPodCount := 0
	unreadyPods := sets.NewString()
	missingPods := sets.NewString()

	for _, pod := range podList.Items {
		podSum := int64(0)
		for _, container := range pod.Spec.Containers {
			if containerRequest, ok := container.Resources.Requests[resource]; ok {
				podSum += containerRequest.MilliValue()
				log.Printf("GetResourceReplicas: container %s req:%v  -> podSum=%v\n", container.Name, containerRequest.MilliValue(), podSum)
			} else {
				return 0, 0, time.Time{}, fmt.Errorf("missing request for %s on container %s in pod %s/%s", resource, container.Name, namespace, pod.Name)
			}
		}

		requests[pod.Name] = podSum
		log.Printf("total for %s: %v\n", pod.Name, podSum)

		if pod.Status.Phase != apiv1.PodRunning || !podv1.IsPodReady(&pod) {
			// save this pod name for later, but pretend it doesn't exist for now
			unreadyPods.Insert(pod.Name)
			delete(metrics, pod.Name)
			continue
		}

		if _, found := metrics[pod.Name]; !found {
			// save this pod name for later, but pretend it doesn't exist for now
			missingPods.Insert(pod.Name)
			continue
		}

		readyPodCount++
	}

	if len(metrics) == 0 {
		return 0, 0, time.Time{}, fmt.Errorf("did not receive metrics for any ready pods")
	}

	usageRatio, utilization, _, err := metricsclient.GetResourceUtilizationRatio(metrics, requests, targetUtilization)
	log.Printf("usageRatio: %v, utilization: %v\n", usageRatio, utilization)
	if err != nil {
		return 0, 0, time.Time{}, err
	}

	rebalanceUnready := len(unreadyPods) > 0 && usageRatio > 1.0
	if !rebalanceUnready && len(missingPods) == 0 {
		if math.Abs(1.0-usageRatio) <= tolerance {
			// return the current replicas if the change would be too small
			return currentReplicas, utilization, timestamp, nil
		}

		// if we don't have any unready or missing pods, we can calculate the new replica count now
		return int32(math.Ceil(usageRatio * float64(readyPodCount))), utilization, timestamp, nil
	}

	if len(missingPods) > 0 {
		if usageRatio < 1.0 {
			// on a scale-down, treat missing pods as using 100% of the resource request
			for podName := range missingPods {
				metrics[podName] = requests[podName]
			}
		} else if usageRatio > 1.0 {
			// on a scale-up, treat missing pods as using 0% of the resource request
			for podName := range missingPods {
				metrics[podName] = 0
			}
		}
	}

	if rebalanceUnready {
		// on a scale-up, treat unready pods as using 0% of the resource request
		for podName := range unreadyPods {
			metrics[podName] = 0
		}
	}

	// re-run the utilization calculation with our new numbers
	newUsageRatio, _, _, err := metricsclient.GetResourceUtilizationRatio(metrics, requests, targetUtilization)
	log.Printf("newUsageRatio: %v\n", newUsageRatio)
	if err != nil {
		return 0, utilization, time.Time{}, err
	}

	if math.Abs(1.0-newUsageRatio) <= tolerance || (usageRatio < 1.0 && newUsageRatio > 1.0) || (usageRatio > 1.0 && newUsageRatio < 1.0) {
		// return the current replicas if the change would be too small,
		// or if the new usage ratio would cause a change in scale direction
		return currentReplicas, utilization, timestamp, nil
	}

	// return the result, where the number of replicas considered is
	// however many replicas factored into our calculation
	return int32(math.Ceil(newUsageRatio * float64(len(metrics)))), utilization, timestamp, nil
}

// GetMetricReplicas calculates the desired replica count based on a target resource utilization percentage
// of the given resource for pods matching the given selector in the given namespace, and the current replica count
func (c *ReplicaCalculator) GetMetricReplicas(currentReplicas int32, targetUtilization float64, metricName string, namespace string, labelsSelector labels.Selector) (replicaCount int32, utilization float64, timestamp time.Time, err error) {
	selector := labelsSelector.String()
	metrics, timestamp, err := c.metricsClient.GetRawMetric(metricName, namespace, labelsSelector)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("unable to get metric  %s: %v", metricName, err)
	}

	podList, err := c.podsGetter.Pods(namespace).List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("unable to get pods while calculating replica count: %v", err)
	}

	if len(podList.Items) == 0 {
		return 0, 0, time.Time{}, fmt.Errorf("no pods returned by selector while calculating replica count")
	}

	readyPodCount := 0
	unreadyPods := sets.NewString()
	missingPods := sets.NewString()

	for _, pod := range podList.Items {
		if pod.Status.Phase != apiv1.PodRunning || !podv1.IsPodReady(&pod) {
			// save this pod name for later, but pretend it doesn't exist for now
			unreadyPods.Insert(pod.Name)
			delete(metrics, pod.Name)
			continue
		}

		if _, found := metrics[pod.Name]; !found {
			// save this pod name for later, but pretend it doesn't exist for now
			missingPods.Insert(pod.Name)
			continue
		}

		readyPodCount++
	}

	if len(metrics) == 0 {
		return 0, 0, time.Time{}, fmt.Errorf("did not recieve metrics for any ready pods")
	}

	usageRatio, tmp_utilization := metricsclient.GetMetricUtilizationRatio(metrics, int64(targetUtilization))
	utilization = float64(tmp_utilization) // emulate newer version of metrcisclient
	if err != nil {
		return 0, 0, time.Time{}, err
	}

	rebalanceUnready := len(unreadyPods) > 0 && usageRatio > 1.0

	if !rebalanceUnready && len(missingPods) == 0 {
		if math.Abs(1.0-usageRatio) <= tolerance {
			// return the current replicas if the change would be too small
			return currentReplicas, utilization, timestamp, nil
		}

		// if we don't have any unready or missing pods, we can calculate the new replica count now
		return int32(math.Ceil(usageRatio * float64(readyPodCount))), utilization, timestamp, nil
	}

	if len(missingPods) > 0 {
		if usageRatio < 1.0 {
			// on a scale-down, treat missing pods as using 100% of the resource request
			for podName := range missingPods {
				metrics[podName] = int64(targetUtilization)
			}
		} else {
			// on a scale-up, treat missing pods as using 0% of the resource request
			for podName := range missingPods {
				metrics[podName] = 0
			}
		}
	}

	if rebalanceUnready {
		// on a scale-up, treat unready pods as using 0% of the resource request
		for podName := range unreadyPods {
			metrics[podName] = 0
		}
	}

	// re-run the utilization calculation with our new numbers
	newUsageRatio, _ := metricsclient.GetMetricUtilizationRatio(metrics, int64(targetUtilization))
	if err != nil {
		return 0, utilization, time.Time{}, err
	}

	if math.Abs(1.0-newUsageRatio) <= tolerance || (usageRatio < 1.0 && newUsageRatio > 1.0) || (usageRatio > 1.0 && newUsageRatio < 1.0) {
		// return the current replicas if the change would be too small,
		// or if the new usage ratio would cause a change in scale direction
		return currentReplicas, utilization, timestamp, nil
	}

	// return the result, where the number of replicas considered is
	// however many replicas factored into our calculation
	return int32(math.Ceil(newUsageRatio * float64(len(metrics)))), utilization, timestamp, nil
}

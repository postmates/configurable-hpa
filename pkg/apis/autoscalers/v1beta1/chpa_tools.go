/*
Copyright 2018 Ivan Glushkov.

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

package v1beta1

import (
	"fmt"
)

func (chpa CHPA) String() string {
	ret := fmt.Sprintf("Spec:%v Status:%v", chpa.Spec, chpa.Status)
	return ret
}

func (chpa_spec CHPASpec) String() string {
	minReplicas := "<nil>"
	if chpa_spec.MinReplicas != nil {
		minReplicas = fmt.Sprintf("%v", *chpa_spec.MinReplicas)
	}
	metrics := "[]" // Map(chpa_spec.Metrics, func(metric scalev2b1.MetricSpec

	ret := fmt.Sprintf("{Ref:%v/%v DFWS:%v UFWS:%v SULF:%v SULM:%v T:%v MinR:%v MaxR:%v M:%s}",
		chpa_spec.ScaleTargetRef.Kind,
		chpa_spec.ScaleTargetRef.Name,
		chpa_spec.DownscaleForbiddenWindowSeconds,
		chpa_spec.UpscaleForbiddenWindowSeconds,
		chpa_spec.ScaleUpLimitFactor,
		chpa_spec.ScaleUpLimitMinimum,
		chpa_spec.Tolerance,
		minReplicas,
		chpa_spec.MaxReplicas,
		metrics)
	return ret
}

func (chpa_status CHPAStatus) String() string {
	curCPU := "<nil>"
	lastScaleTime := "<nil>"
	if chpa_status.LastScaleTime != nil {
		lastScaleTime = fmt.Sprintf("%v", *chpa_status.LastScaleTime)
	}
	ret := fmt.Sprintf("{LST:%v CR:%v DR:%v CCPUUP:%v}", lastScaleTime, chpa_status.CurrentReplicas, chpa_status.DesiredReplicas, curCPU)
	return ret
}

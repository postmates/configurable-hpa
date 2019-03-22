# Configurable HPA

**WARNING**: If you want to delete your CHPA, do it carefully not to remove your deployment too. Read the ["Quick Start Guide"](QuickStartGuide.md).

**WARNING**: You should remove usual HPA before starting using CHPA. If you use both, the behaviour is undefined (they'll fight each other).

Vanilla kubernetes [HPA (Horizontal Pod Autoscaler)](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) in 1.11 version doesn't allow to configure some HPA parameters, such as:

 - [DownscaleForbiddenWindow](https://github.com/kubernetes/website/blob/snapshot-initial-v1.11/content/en/docs/tasks/run-application/horizontal-pod-autoscale.md#support-for-cooldowndelay)
 - [UpscaleForbiddenWindow](https://github.com/kubernetes/website/blob/snapshot-initial-v1.11/content/en/docs/tasks/run-application/horizontal-pod-autoscale.md#support-for-cooldowndelay)
 - Tolerance
 - ScaleUpLimit parameters (ScaleUpLimitFactor and ScaleUpLimitMinimum).

These parameters are specified either a cluster-wide, or hardcoded into the HPA code.

For more info about how HPA in v1.10.8 works and what these parameters means check [the internal sig-autoscaling document](https://docs.google.com/document/d/1Gy90Rbjazq3yYEUL-5cvoVBgxpzcJC9vcfhAkkhMINs/edit#),

This becomes a problem for us as we need to have some Services scaling up really fast and at the same time we need some Services scaling "as usual".
So we implemented a [CRD (Custom Resource Definition)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions)
and a corresponding controller that will mimic vanilla HPA, and will be flexibly configurable.

The skeleton of the controller is created with the help of [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).

## CHPA Algorithm

Configurable HPA (CHPA) controller starts every 15 seconds, on every iteration it follows the instruction:

* check all CHPA objects
* for every CHPA object:
  * find the correspondent Deployment object
  * check metrics for all the Containers for all the Pods of the Deployment object
  * calculate the desired number of Replicas (terms Replicas and Pods mean the same in CHPA context)
  * adjust Replica Number

## CHPA Parameters

Each CHPA object can have the following parameters set:

* **UpscaleForbiddenWindowSeconds** - is the duration window from the previous `ScaleUp` event
    for the particular CHPA object when we won't try to ScaleUp again
* "Scale Up Limit" parameters (**ScaleUpLimitFactor** and **ScaleUpLimitMinimum**) limit the number of replicas for the next `ScaleUp` event.

    If the Pods metrics show that that we should increase number of replicas,
    the algorithm will try to limit the increase by the `ScaleUpLimit`

    `ScaleUpLimit` is found as a maximum of an absolute number (`ScaleUpLimitMinimum`) and
    of a multiplication of currentReplicas by a coefficient (`ScaleUpLimitFactor`):

```
    ScaleUpLimit = max(ScaleUpLimitMinimum, ScaleUpLimitFactor * currentReplicas)
    NextReplicas = min(ScaleUpLimit, DesiredReplicas)
```

* **DownscaleForbiddenWindowSeconds** - the same as `UpscaleForbiddenWindowSeconds`
    but for `ScaleDown`
* **Tolerance** - how sensitive CHPA to the metrics change. Default value is `0.1`.

    E.g. if

    `Math.abs(1 - RealUtilization/TargetUtilization) < Tolerance`

    Then the CHPA won't change number of replicas.
    Use with care!

## Configuration Examples

`currentReplicas = 1, ScaleUpLimitMinimum = 4, ScaleUpLimitFactor = 2`

* => ScaleUpLimit = 4
* i.e. if metrics shows that we should scale up to 10 Replicas, we'll scale up to 4 Replicas
* i.e. if metrics shows that we should scale up to 3 Replicas, we'll scale up to 3 Replicas

`currentReplicas = 10, ScaleUpLimitMinimum = 4, ScaleUpLimitFactor = 3`
* => ScaleUpLimit = 30
* i.e. if metrics shows that we should scale up to 10 Replicas, we'll scale up to 10 Replicas
* i.e. if metrics shows that we should scale up to 40 Replicas, we'll scale up to 30 Replicas

## Investigate problems

There're two places where you can check problems with your CHPA:

- CHPA object itself. It contains "Events" and "Conditions" that are filled by the CHPA controller. In case of any problem with the CHPA you should check these fields.

    kubectl describe chpas.autoscalers.postmates.com chpa-example1

- CHPA controller logs. The logs may contain information about controller problems (couldn't connect to the server, etc)

    stern -n kube-system configurable-hpa

# Development

To perform development you have to store the sources on the following path

    $GOPATH/src/github.com/postmates/configurable-hpa

To run tests you need to have [kubebuilder](https://book.kubebuilder.io/) installed:

    make test

To run e2e test you need to have a kubectl in your `$PATH` and have kubectl context configured.

The test will create several Deployments and Services, prepare some load for them and check that the Deployment is scaled

    make e2e

WARNING: Do not stop the test otherwise it won't be able to clean all the test resources automatically.

To manually clean all the resources for the tests:

    kubectl delete service,deploy,chpas.autoscalers.postmates.com -l app=chpa-test

NB: RBAC configs in `config/rbac` are autogenerated and should be used as a draft for your Kubernetes installation.

# License

Configurable-HPA is copyright © 2019 Postmates, Inc and released to the public under the terms of the MIT license.

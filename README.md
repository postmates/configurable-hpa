# Configurable HPA

NB: The work is still in progress

WARNING: If you want to delete your CHPA, do it carefully not to remove your deployment too. Read the "Quick Start Guide" below.

WARNING: You should remove usual HPA before starting using CHPA. If you use both, the behaviour is undefined (they'll fight each other).

Vanilla kubernetes [HPA (Horizontal Pod Autoscaler)](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) doesn't allow to configure some HPA parameters, such as:

 - [DownscaleForbiddenWindow](https://github.com/kubernetes/website/blob/snapshot-initial-v1.11/content/en/docs/tasks/run-application/horizontal-pod-autoscale.md#support-for-cooldowndelay)
 - [UpscaleForbiddenWindow](https://github.com/kubernetes/website/blob/snapshot-initial-v1.11/content/en/docs/tasks/run-application/horizontal-pod-autoscale.md#support-for-cooldowndelay)
 - Tolerance
 - ScaleUpLimit parameters (ScaleUpLimitFactor and ScaleUpLimitMinimum). 

These parameters are specified either a cluster-wide, or hardcoded into the HPA code.

For more info about how HPA in v1.10.8 works and what these parameters means check [the internal sig-autoscaling document](https://docs.google.com/document/d/1Gy90Rbjazq3yYEUL-5cvoVBgxpzcJC9vcfhAkkhMINs/edit#), 

This becomes a problem for us as we need to have some Services scaling up really fast (i.e. IPA during promos).
So we implemented a [CRD (Custom Resource Definition)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) 
and a corresponding controller that will mimic vanilla HPA, and will be flexibly configurable.


# Quick Start Guide

## Create a deployment

Let's start a deployment `chpa-example`, that will imitate your real application:

    kubectl run chpa-example --image=k8s.gcr.io/hpa-example --requests=cpu=200m --expose --port=80

## Create a CHPA

Then let's create a CHPA manifest that'll specify forbidden windows, 
min and max replicas, and our deployment name.

```bash
cat > chpa.yaml << EOF
apiVersion: autoscalers.postmates.com/v1beta1
kind: CHPA
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: chpa-example
spec:
  downscaleForbiddenWindowSeconds: 15
  upscaleForbiddenWindowSeconds: 15
  scaleTargetRef:
    kind: Deployment
    name: chpa-example
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      targetAverageUtilization: 50
EOF
```

Let's apply the manifest:

    kubectl apply -f chpa.yaml

NB: the deployment and the chpa for that deployment should be started in the same namespace!

## Add some load

Now, let's add some load for our deployment to check how it will scale:

```
$ kubectl run -i --tty my-load-generator --image=busybox /bin/sh
/ #     # we are in the k8s container, let's create some load
/ # while true; do wget -q -O- chpa-example; done;
OK!OK!OK!OK!OK!OK!OK!...
```

## Check how deployment scales

Run in a separate shell:

```
❯ kubectl get deploy chpa-example -w
NAME            DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
chpa-example   1         1         1            1           1h55m
chpa-example   4     1     1     1     1h55m
chpa-example   4     1     1     1     1h55m
chpa-example   4     1     1     1     1h55m
chpa-example   4     4     4     1     1h55m
chpa-example   4     4     4     2     1h56m
chpa-example   4     4     4     3     1h56m
chpa-example   4     4     4     4     1h56m
chpa-example   7     4     4     4     1h57m
chpa-example   7     4     4     4     1h57m
chpa-example   7     4     4     4     1h57m
chpa-example   7     7     7     4     1h57m
chpa-example   7     7     7     5     1h57m
chpa-example   7     7     7     6     1h58m
chpa-example   7     7     7     7     1h58m
chpa-example   1     7     7     7     1h59m
chpa-example   1     7     7     7     1h59m
chpa-example   1     1     1     1     1h59m
```

As you can see, the deployment scaled up from 1 to 7 instances in 2 minutes.
Then it scaled down to 1 replicas again.

That would be impossible with the vanilla HPA, where `ScaleUpForbiddenWindow` is 3min and `ScaleDownForbiddenWindow` is 5min.

## Delete the CHPA

If you decided to stop using the CHPA, you should carefully remove the CHPA without removing the 
deployment itself. To do it just add `--cascade=false` parameter to the `kubect delete` command:

    kubectl delete chpas.autoscalers.postmates.com chpa-example --cascade=false

The thing is that CHPA is registered as an [Owner](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#ownerreference-v1-meta) for the deployment.
When we delete the owner of the deployment, the deployment is garbage collected.

## Clean everything else

We don't want to leave the garbage behind

```bash
kubectl delete deploy/my-load-generator deploy/chpa-example
```

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

To run e2e test you need to have a kubectl in your `$PATH` and have 
kubectl context configured. 
The test will create several Deployments and Services, prepare some load for them and check that the Deployment is scaled

    make e2e

WARNING: Do not stop the test otherwise it won't be able to clean all the test resources automatically.

To manually clean all the resources for the tests:

    kubectl delete service,deploy,chpas.autoscalers.postmates.com -l app=chpa-test

NB: RBAC configs in `config/rbac` are autogenerated and used as a draft for [pi-k8s PR](https://github.com/postmates/pi-k8s/pull/1811). You SHOULD NOT use it!

# Deployment

For now the [DroneCI pipeline](https://ci.admin.us-east-2.postmates.com/postmates/configurable-hpa)
is used to run unit tests and to install CHPA controller to the `admin` cluster. 
`e2e` testing should be performed manually  after each deployment to the `admin`/`stage` cluster.

Install to the `stage` cluster, send the following message in the `#team-infra` channel:

    @Deploybot configurable-hpa 34 stage

where `34` is a build number in the DroneCI pipeline for master that have all the tests pass.

Install to the `prod` cluster:

    @Deploybot configurable-hpa 34 prod

To get chpa-controller logs:

    stern -n kube-system configurable-hpa

# TODO

Here's a list of things that must be done next:

- ScaleUpLimitPods + ScaleUpLimitPercentages
- tests for scaleuplimit*
- Check how to deal with CHPA version change v1beta1 -> v1beta2
- (done) tests for incorrect chpa spec + check that if you fix it - chpa works
- (done) catch controller exception
- (done) Switch from v1.5.8 to v1.10.8 for replica_calculator to use the newest metrics-server functionality
- (done) Check RBAC rules
- (done) Check how several chpa objects works together
- (done) Check OWNERship (solution: add `--cascade=false` parameter)
- (done) Fix exponential backoff for reconcile queue
- (done) "FailedGetMetrics: unable to get metrics for resource cpu: no metrics returned from resource metrics API"
- (done) Add unittests to the CI pipeline
- (done) Add e2e tests
- (done) Add "events" system to the chpa (as in hpa) to show problems/events with each particular deployment scaling process
- (done) Properly log the chpa controller activity

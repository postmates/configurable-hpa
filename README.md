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

# Development

To make tests work you need to have [kubebuilder](https://book.kubebuilder.io/) installed

To install CHPA to k8s for now manual helm is used (to be added to the CI pipeline later)

Install to the `admin` cluster:

    helm --kube-context admin.us-east-2.aws.k8s --tiller-namespace kube-system upgrade configurable-hpa deployment/helm/configurable-hpa --install --namespace kube-system --wait=true --values deployment/environments/ci/values.yaml --set=image.tag=v1beta1-9-3cf57c22 --debug

Install to the `stage` cluster:

    helm --kube-context stage.us-west-2.aws.k8s --tiller-namespace kube-system upgrade configurable-hpa deployment/helm/configurable-hpa --install --namespace kube-system --wait=true --values deployment/environments/stage/values.yaml --set=image.tag=v1beta1-9-3cf57c22 --debug

To get chpa-controller logs:

    stern -n kube-system configurable-hpa

NB: RBAC configs in `config/rbac` are autogenerated and used as a draft for [pi-k8s PR](https://github.com/postmates/pi-k8s/pull/1811). You SHOULD NOT use it!

# TODO

Here's a list of things that must be done next:

- (done) Switch from v1.5.8 to v1.10.8 for replica_calculator to use the newest metrics-server functionality
- (done) Check RBAC rules
- (done) Check how several chpa objects works together
- (done) Check OWNERship (solution: add `--cascade=false` parameter)
- Fix exponential backoff for reconcile queue
- "FailedGetMetrics: unable to get metrics for resource cpu: no metrics returned from resource metrics API"
- Add unittests to the CI pipeline
- Add e2e tests
- Add "events" system to the chpa (as in hpa) to show problems/events with each particular deployment scaling process
- Add more checks into `isHPAValid` and `isHPASpecValid`
- Add check that HPA is used for this particular Deployment/ReplicaSet before applying the CHPA
- Check how to deal with CHPA version change v1beta1 -> v1beta2
- (probably) log

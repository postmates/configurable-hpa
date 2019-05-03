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

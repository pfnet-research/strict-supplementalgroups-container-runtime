# Getting Started

This can test _strict-supplementalgroups-container-runtime_ locally.

## Setup

This creates a kind cluster, installs this container runtime and `strict-supplementalgroups` Runtime Class to the cluster.

```shell
$ cd getting-started
$ ./setup.sh
```

## Try

### Check docker image can bypass SupplementalGroups easily

```shell
$ cat <<EOT | kubectl apply -f - && kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/bypass-suppplemtalgroups-pod
# this pod used 'bypass-supplementalgroups-image' 
# which can bypass supplemental groups
apiVersion: v1
kind: Pod
metadata:
    name: bypass-suppplemtalgroups-pod
spec:
  securityContext:
    # Suppose this security context is enforced by PSP
    # see ../README for detailed scenario
    runAsUser: 1000
    runAsGroup: 1000
    supplementalGroups: [60000]
  restartPolicy: Never
  containers:
  - image: bypass-supplementalgroups-image
    name: main
    command: ["bash", "-c", "id; true"]
    imagePullPolicy: Never
EOT

# You see the proceess can bypass supplementalGroups
# by defining bypass group belongings in the container image
# (It belongs to gid=50000(bypassed-group))
$ kubectl logs bypass-suppplemtalgroups-pod
uid=1000(alice) gid=1000(alice) groups=1000(alice),50000(bypassed-group),60000
```

### With `strict-supplementalgroups` Runtime Class

`strict-supplementalgroups` Runtime Class can enforces `supplementalGroups` to the container for avoid bypassing it by the container image.

```shell
$ cat <<EOT | kubectl apply -f - && kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/strict-suppplemtalgroups-pod
# this pod used 'strict-supplementalgroups-image'.
# But use strict-supplementalgroups RuntimeClass
# which enforces supplementalGroups configuration
apiVersion: v1
kind: Pod
metadata:
    name: strict-suppplemtalgroups-pod
spec:
  runtimeClassName: strict-supplementalgroups
  securityContext:
    # Suppose this security context is enforced by PSP
    # see ../README for detailed scenario
    runAsUser: 1000
    runAsGroup: 1000
    supplementalGroups: [60000]
  restartPolicy: Never
  containers:
  - image: bypass-supplementalgroups-image
    name: main
    command: ["bash", "-c", "id; true"]
    imagePullPolicy: Never
EOT

# You see the process can NOT bypass supplementalGroups by the RuntimeClass
# even if the container image defines bypass group belongings.
# (It does NOT belongs to gid=50000(bypassed-group))
$ kubectl logs strict-suppplemtalgroups-pod
uid=1000(alice) gid=1000(alice) groups=1000(alice),60000
```

## Clean Up

```shell
kind delete cluster
```

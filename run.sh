#! /usr/bin/env bash

set -eu

MODE=${MODE:-containerd}
if [ "${MODE}" != "containerd" ] && [ "${MODE}" != "cri-o" ]; then
    echo "MODE must be either containerd or cri-o" 1>&2
    exit 1
fi

echo "reproducing bypass supplementalGroups by docker image..."

KIND_CLUSTER_NANE=reproduce-bypass-supplementalgroups

kind -q delete cluster --name ${KIND_CLUSTER_NANE} || true
case "${MODE}" in
    "containerd")
        kind -q create cluster --name ${KIND_CLUSTER_NANE} --config cluster-config.yaml
        ;;
    "cri-o")
        docker build -q . -t kindnode/cri-o:1.25.2 -f cri-o/Dockerfile > /dev/null
        kind -q create cluster --name ${KIND_CLUSTER_NANE} --config cri-o/cluster-config.yaml
        ;;
esac

docker build -q . -t  bypass-supplementalgroups-image:latest > /dev/null
case "${MODE}" in
    "containerd")
        kind -q load docker-image bypass-supplementalgroups-image:latest --name ${KIND_CLUSTER_NANE}
        ;;
    "cri-o")
        docker save bypass-supplementalgroups-image:latest > cri-o/bypass-supplementalgroups-image-latest.tar
        docker exec "${KIND_CLUSTER_NANE}-control-plane" skopeo copy docker-archive:/host/bypass-supplementalgroups-image-latest.tar containers-storage:docker.io/bypass-supplementalgroups-image:latest >/dev/null 2>&1
        rm cri-o/bypass-supplementalgroups-image-latest.tar
        ;;
esac

until kubectl get serviceaccount default >/dev/null 2>&1; do sleep 1; done
kubectl apply -f pod.yaml > /dev/null
kubectl wait --timeout=3m --for=jsonpath='{.status.phase}'=Succeeded pod bypass-supplementalgroups-pod > /dev/null

echo ""
echo "==========================================================================="
echo "versions"
echo "==========================================================================="
echo "kind version"
kind version
echo ""
case "${MODE}" in
    "containerd")
        echo "containerd version"
        docker exec ${KIND_CLUSTER_NANE}-control-plane containerd -v 
        ;;
    "cri-o")
        echo "cri-o version"
        docker exec ${KIND_CLUSTER_NANE}-control-plane crio -v 
        ;;
esac
echo ""
echo "kubectl version"
kubectl version --short 2>/dev/null
echo ""
echo "OS version"
echo "$ cat /etc/os-release"
docker exec ${KIND_CLUSTER_NANE}-control-plane cat /etc/os-release
echo "$ uname -a"
docker exec ${KIND_CLUSTER_NANE}-control-plane uname -a

echo ""
echo "==========================================================================="
echo "bypass-supplementalgroups-pod manifest"
echo "  supplementalGroups: [60000]"
echo "==========================================================================="
cat pod.yaml

echo ""
echo "==========================================================================="
echo "Dockerfile of bypass-supplementalgroups-image"
echo "  alice(uid=10000) belongs to bypassed-group(gid=50000)"
echo "==========================================================================="
cat Dockerfile

echo ""
echo "==========================================================================="
echo "Actuall groups of bypass-supplementalgroups-pod's are bypassed supplementalGroups"
echo "  alice(uid=1000) can belong to both "
echo "  - 50000(bypassed: NOT IN supplementalGroups, but in container image"
echo "  - 60000(in supplementalGroups)"
echo "==========================================================================="
kubectl logs bypass-supplementalgroups-pod

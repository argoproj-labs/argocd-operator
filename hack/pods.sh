#!/bin/sh

set -e

kubectl get po -n $1
pods=$(kubectl get po -n $1 -o=name)
for pod in $pods; do
    echo "===================="
    kubectl describe -n $1 $pod
done
echo "===================="
kubectl get secrets -n $1

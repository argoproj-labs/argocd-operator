#!/bin/bash

# Loop forever, running df -h and sleeping for 5 seconds
while true; do
    echo "--------------------"
    kubectl get namespaces
    echo "----"
    df -h | grep root
    sleep 10
done

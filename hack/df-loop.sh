#!/bin/bash

# Loop forever, running df -h and sleeping for 5 seconds
while true; do
    df -h
    sleep 5
done

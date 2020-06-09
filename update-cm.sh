#!/bin/bash

kubectl delete configmap $1
kubectl create configmap $1 --from-file=$2

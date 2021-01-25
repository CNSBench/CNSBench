#!/bin/bash

# Tears down a minikube cluster, starts a new one, installs
# cnsbench components, installs quickstart resources, runs
# a benchmark.  Check that a benchmark pod gets created in
# the default namespace after this script completes, and
# look for output of cnsbench-output-collector in the
# cnsbench-system namespace.

if minikube status | grep "apiserver: Running"; then
	read -p "Delete running minikube cluster? " -r -n1
	echo
	if [[ ! $REPLY =~ ^[Yy]$ ]]; then
		echo "OK, exiting without doing anything"
		exit 0
	fi

	echo -n "Deleting minikube cluster in "
	for i in {10..1}; do
		echo -n "$i..."
		sleep 1
	done
	echo
fi

minikube delete
minikube start
sleep 10
minikube status
minikube ssh "sudo mkdir -p /mnt/sda1/data/pv1"
sed -i 's/path:.*/path: \/mnt\/sda1\/data\/pv1/' doc/examples/quickstart/pv.yaml
sed -i 's/-$/- minikube/' doc/examples/quickstart/pv.yaml
make deploy-from-manifests
sleep 10
kubectl get pods -ncnsbench-system
kubectl apply -f doc/examples/quickstart/local-sc.yaml
kubectl apply -f doc/examples/quickstart/pv.yaml
kubectl apply -f doc/examples/quickstart/benchmark.yaml

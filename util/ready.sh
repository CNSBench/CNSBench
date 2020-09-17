#!/bin/bash

# $1 = querystring
# $2 = number of containers need to be finished
# $3 = (optional) syncgroup string

echo $1
echo $2

d=0
while [[ $d -lt $2 ]]; do
	sleep 5
	d=`curl -k -X GET  -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" https://$KUBERNETES_PORT_443_TCP_ADDR:$KUBERNETES_SERVICE_PORT_HTTPS/api/v1/namespaces/default/pods?labelSelector=$1 | jq .items[].status.initContainerStatuses | grep reason.*Completed | wc -l`
	echo $d
done

if [[ "$#" -gt 2 ]]; then
	numneeded=`curl -k -X GET  -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" https://$KUBERNETES_PORT_443_TCP_ADDR:$KUBERNETES_SERVICE_PORT_HTTPS/api/v1/namespaces/default/pods?labelSelector=$3 | jq '[.items[].spec | select(has("initContainers") != false)] | .[].initContainers[].name' | grep -v sync-container | wc -l`
	d=0

	while [[ $d -lt $numneeded ]]; do
		sleep 5
		d=`curl -k -X GET  -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" https://$KUBERNETES_PORT_443_TCP_ADDR:$KUBERNETES_SERVICE_PORT_HTTPS/api/v1/namespaces/default/pods?labelSelector=$3 | jq .items[].status.initContainerStatuses | grep reason.*Completed | wc -l`
		echo $d
	done
fi

#!/bin/sh

# $1 = number of containers that can still be running
# (2 for the parser container, 1 for the output container)

echo $1
echo $POD_NAME

numcontainers=`curl -k -X GET  -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/podwatcher/token)" https://$KUBERNETES_PORT_443_TCP_ADDR:$KUBERNETES_SERVICE_PORT_HTTPS/api/v1/namespaces/default/pods/$POD_NAME | jq .status.containerStatuses | grep name | wc -l`
echo "NUM CONTAINERS $numcontainers"
numneeded=$(( $numcontainers - $1 ))
echo "NUM NEEDED $numneeded"

d=$(( $numneeded + 1 ))
while [[ $d -gt $numneeded ]]; do
  sleep 5
  d=`curl -k -X GET  -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/podwatcher/token)" https://$KUBERNETES_PORT_443_TCP_ADDR:$KUBERNETES_SERVICE_PORT_HTTPS/api/v1/namespaces/default/pods/$POD_NAME | jq .status.containerStatuses | grep running | wc -l`
  echo $d
done

echo "DONE"

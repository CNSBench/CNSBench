#!/bin/sh

# $1 = filename
# $2 = number of containers that need to be finished

echo $1
echo $2
echo $3

d=0
while [[ $d -lt $2 ]]; do
        sleep 5
        d=`curl -k -X GET  -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" https://$KUBERNETES_PORT_443_TCP_ADDR:$KUBERNETES_SERVICE_PORT_HTTPS/api/v1/namespaces/default/pods/$POD_NAME | jq .status.containerStatuses | grep reason.*Completed | wc -l`
        echo $d
done

ls -l /output
/parser/* $1

exit 0

Prerequisites
-------------

* Install v0.18 of the Operator SDK and CLI tool: https://v0-18-x.sdk.operatorframework.io/docs/install-operator-sdk/
* Install additional prerequisites for Golang based Operators: https://v0-18-x.sdk.operatorframework.io/docs/golang/installation/

Setup
-----

* Install the custom resource definition for the Benchmark object:
`kubectl apply -f deploy/crds/cnsbench.example.com_benchmarks_crd.yaml`
* Clone and install the Workload Library (https://github.com/CNSBench/workload-library)

Running
-------

Make sure the KUBECONFIG environment variable is set, e.g.
`export KUBECONFIG=~/.kube/config`

Run the controller with `operator-sdk run local`

Currently the only type of output implemented is the "HTTP POST" output.  To
get started quickly, you can run a simple Python HTTP echo server that will
simply print the benchmark output to stdout, e.g.:

```python
from http.server import HTTPServer, BaseHTTPRequestHandler

class RequestHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        print(self.rfile.read(int(self.headers.get('Content-Length'))))
        self.send_response(200)

server = HTTPServer(('', 8888), RequestHandler)
server.serve_forever()
```

To run a benchmark, define the benchmark in a yaml file and instantiate it with `kubectl apply -f`.
See `samples/` for sample benchmark resource definitions.

Once a benchmark resource has been created, use the standard `kubectl get/describe` to see its status.

Troubleshooting
---------------

If for some reason the controller crashes and a Benchmark resource still has a
finalizer set, it is safe to manually remove the finalizer so that the resource
can be deleted.  To do so, do `kubectl edit benchmark <resource name>` and
delete the line containing the finalizer.

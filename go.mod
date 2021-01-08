module github.com/cnsbench/cnsbench

go 1.15

require (
	github.com/elastic/go-elasticsearch/v7 v7.10.0
	github.com/go-logr/logr v0.3.0
	github.com/kubernetes-csi/external-snapshotter/v2 v2.1.3
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/apiserver v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800
	sigs.k8s.io/controller-runtime v0.7.0
	sigs.k8s.io/controller-tools v0.4.1 // indirect
)

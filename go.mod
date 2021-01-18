module github.com/cnsbench/cnsbench

go 1.15

require (
	github.com/elastic/go-elasticsearch/v7 v7.10.0
	github.com/go-logr/logr v0.3.0
	github.com/kubernetes-csi/external-snapshotter/v2 v2.1.3
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/tools v0.0.0-20200616195046-dc31b401abb5 // indirect
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/apiserver v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/utils v0.0.0-20200912215256-4140de9c8800
	sigs.k8s.io/controller-runtime v0.7.0
)

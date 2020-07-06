module github.com/cnsbench

go 1.14

require (
	github.com/kubernetes-csi/external-snapshotter v1.2.2 // indirect
	github.com/kubernetes-csi/external-snapshotter/v2 v2.1.1
	github.com/operator-framework/operator-sdk v0.18.2
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.18.5
	k8s.io/apimachinery v0.18.5
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.0
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.3.0+incompatible

replace k8s.io/client-go => k8s.io/client-go v0.18.5

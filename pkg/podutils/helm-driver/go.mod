module helm-driver

go 1.16

require (
	github.com/cnsbench/cnsbench v0.0.0-20210429123313-9d14ce66defb
	helm.sh/helm/v3 v3.5.4
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v0.20.5
	sigs.k8s.io/controller-runtime v0.8.3
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
)

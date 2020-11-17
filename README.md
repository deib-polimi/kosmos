![pipelines](https://github.com/lterrac/system-autoscaler/workflows/base-pipeline/badge.svg)

# system-autoscaler
Kubernetes components embedding vertical and horizontal container resource scaling

# Controller guidelines 

The controllers are freely inspired from [sample-controller](https://github.com/kubernetes/sample-controller)

# Components
- [Contention Manager](pkg/contention-manager/README.md)
- [Pod Replicas Updater](pkg/pod-replicas-updater/README.md)
- [Pod Resource Updater](pkg/pod-resource-updater/README.md)
- [Podscale Controller](pkg/podscale-controller/README.md)
- [Recommender](pkg/recommender/README.md)

# CRDs code generation

Since the API code generator used in [hack/update-codegen.sh](hack/update-codegen.sh) was not designed to work with Go modules, it is mandatory to recreate the entire module path in order to make the code generation work.  
This gives you two options:  
1) Create the folders `github.com/lterrac` and clone this repository in any location of your filesystem.
2) Clone the repository inside the `GOPATH` directory.

In the end there is no choice other than to preserve the module hierarchy.

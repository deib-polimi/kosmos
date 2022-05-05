![pipelines](https://github.com/lterrac/system-autoscaler/workflows/base-pipeline/badge.svg)

# KOSMOS

<p align="center">
  <img width="100%" src="https://i.imgur.com/tm9mSuM.png" alt="Politecnico di Milano" />
</p>

## Overview

KOSMOS is an autoscaling solution, developed at the Politecnico di Milano, for Kubernetes. Pods are individually controlled by control-theoretical planners that manage container resources on-the-fly (vertical scaling). A dedicated component is in charge of handling resource contention scenarios among containers deployed in the same node (a physical or virtual machine). Finally, at the cluster-level a heuristic-based controller is in charge of the horizontal scaling of each application.

# Controller

The controllers are freely inspired from [sample-controller](https://github.com/kubernetes/sample-controller)

- [Contention Manager](pkg/contention-manager/README.md)
- [Pod Replicas Updater](pkg/pod-replicas-updater/README.md)
- [Pod Resource Updater](pkg/pod-resource-updater/README.md)
- [PodScale Controller](pkg/podscale-controller/README.md)
- [Recommender](pkg/recommender/README.md)

# CRDs code generation

Since the API code generator used in [hack/update-codegen.sh](hack/update-codegen.sh) was not designed to work with Go modules, it is mandatory to recreate the entire module path in order to make the code generation work.  
This gives you two options:  
1) Create the folders `github.com/deib-polimi` and clone this repository in any location of your filesystem.
2) Clone the repository inside the `GOPATH` directory.

In the end there is no choice other than to preserve the module hierarchy.

## Citation
If you use this code for evidential learning as part of your project or paper, please cite the following work:  

    @article{baresi2021kosmos,
      title={{KOSMOS:} Vertical and Horizontal Resource Autoscaling for Kubernetes},
      author={Baresi, Luciano and Hu, Davide Yi Xian and Quattrocchi, Giovanni and Terracciano, Luca},
      journal={ICSOC},
      volume={13121},
      pages={821--829},
      year={2021}
    }

## Contributors
* **[Davide Yi Xian Hu](https://github.com/DragonBanana)**
* **[Luca Terracciano](https://github.com/lterrac)**

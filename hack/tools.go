package tools

import (
	_ "k8s.io/code-generator" // This package imports things required by build scripts, to force `go mod` to see them as dependencies
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
)

module github.com/lterrac/system-autoscaler

go 1.15

require (
	github.com/asecurityteam/rolling v2.0.4+incompatible
	github.com/go-logr/logr v0.3.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/stretchr/testify v1.6.1
	golang.org/x/tools v0.0.0-20200616195046-dc31b401abb5 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	k8s.io/api v0.19.4
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v0.19.4
	k8s.io/code-generator v0.19.4
	k8s.io/klog/v2 v2.2.0
	sigs.k8s.io/controller-runtime v0.6.4
)

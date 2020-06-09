// This is a generated file. Do not edit directly.

module github.com/ibmdcb/k8s-serviceroute-conrtoller

go 1.13

require (
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	istio.io/api v0.0.0-20200518203817-6d29a38039bd
	istio.io/client-go v0.0.0-20200528222059-5465d5e00a32
	k8s.io/api v0.18.1
	k8s.io/apimachinery v0.18.1
	k8s.io/client-go v0.18.1
	k8s.io/code-generator v0.0.0-20200519081644-3bc239a9bae4
	k8s.io/klog v1.0.0 // indirect
	k8s.io/klog/v2 v2.0.0
)

replace (
	golang.org/x/sys => golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a // pinned to release-branch.go1.13
	golang.org/x/tools => golang.org/x/tools v0.0.0-20190821162956-65e3620a7ae7 // pinned to release-branch.go1.13
	k8s.io/api => k8s.io/api v0.0.0-20200519082056-2543aba0e237
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20200519081849-bdcc9f4ab675
	k8s.io/client-go => k8s.io/client-go v0.0.0-20200519082352-455d6109ca5a
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20200519081644-3bc239a9bae4
)

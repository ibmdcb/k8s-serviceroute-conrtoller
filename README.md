# serviceroute-controller

This controller controls the lifecyle of a CRD called ServiceRoute. In the example below, for a service called **grafana** in the namespace **grafana-blendstat**, the ServiceRoute declares a route called **grafana-route**.
```yaml
apiVersion: networking.dcb/v1alpha1
kind: ServiceRoute
metadata:
  name: grafana
  namespace: grafana-blendstat
spec:
  routeName: grafana-route
```
The controller acts as the route provider. It monitors the CRD and creates the route using potentially any ingress setup or DNS automation, but in the current implementation, it creates routes on a separate Istio cluster using Virtual Service, Destination Rules, and Ingress Gateway. Upon completion, the controller will update the status with FQDN.
```yaml
apiVersion: networking.dcb/v1alpha1
kind: ServiceRoute
metadata:
  name: grafana
  namespace: grafana-blendstat
spec:
  routeName: grafana-route
status:
  fullRouteName: grafana-route.fqdn.com
```
In essense, it allows an istio cluster to provide **route as a service**, a service needed when manual setup of wildcard dns and certificate takes time.


## Architecture
<p align="center">
  <img src="docs/images/route_as_a_service.jpg"/>
</p>

## Details

This controller uses [sample controller](https://github.com/kubernetes/sample-controller) extensively.

## Build

### When using go 1.11 modules
When using go 1.11 modules (`GO111MODULE=on`), issue the following
commands --- starting in whatever working directory you like.

```sh
git clone https://github.com/ibmdcb/k8s-serviceroute-conrtoller.git
cd k8s-serviceroute-conrtoller
go build -o serviceroute-controller .
```
### Docker
```sh
docker build -t k8s-serviceroute-conrtoller:latest .
```
## Deploy

### On Istio Cluster

* Install Istio 
* Configure Secure Ingress Gateway. The following example references [istio example] (https://archive.istio.io/v1.4/docs/tasks/traffic-management/ingress/secure-ingress-mount/) Newer versions of Istio should work similarly, will test soon.
```yaml
apiVersion: v1
items:
- apiVersion: networking.istio.io/v1alpha3
  kind: Gateway
  metadata:
    name: httpbin-gateway
    namespace: sample
  spec:
    selector:
      istio: ingressgateway
    servers:
    - hosts:
      - abc.domain.com
      port:
        name: http-redirect
        number: 80
        protocol: HTTP
      tls:
        httpsRedirect: true
    - hosts:
      - abc.domain.com
      port:
        name: https
        number: 443
        protocol: HTTPS
      tls:
        mode: SIMPLE
        privateKey: /etc/istio/ingressgateway-certs/tls.key
        serverCertificate: /etc/istio/ingressgateway-certs/tls.crt
```
* Wild card DNS *.domain.com should map to IngressGateway IP
* and Wild Card Cert for *.domain.com should be configured as above

 .:latest
**Prerequisite**: Since the sample-controller uses `apps/v1` deployments, the Kubernetes cluster version should be greater than 1.9.

```sh
# assumes you have a working kubeconfig, not required if operating in-cluster
./serviceroute-controller -kubeconfig=<app-cluster-config> -clustername==<app-cluster-name> -istio_config==<istio-cluster-config> -istio_ns=sample -istio_suffix=<domain.com> -istio_gateway=httpbin-gateway -istio_
gateway_http=http-redirect -istio_gateway_https=https

# create a CustomResourceDefinition
kubectl create -f artifacts/examples/crd.yaml

# create a custom resource of type Foo
kubectl create -f artifacts/examples/example-foo.yaml

# check deployments created through the custom resource
kubectl get deployments
```


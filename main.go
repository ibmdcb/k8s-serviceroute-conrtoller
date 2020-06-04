/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "dell.com/routecontroller/pkg/generated/clientset/versioned"
	informers "dell.com/routecontroller/pkg/generated/informers/externalversions"
	"dell.com/routecontroller/pkg/signals"
	istioclientset "istio.io/client-go/pkg/clientset/versioned"
)

var (
	masterURL    string
	kubeconfig   string
	clustername  string
	istio_config string
	istio        IstioInfo
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	//used to monitor svc and ep in src cluster
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	//used to monitor caps-routes in src cluster
	exampleClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	istiocfg, err := clientcmd.BuildConfigFromFlags(masterURL, istio_config)
	if err != nil {
		klog.Fatalf("Error building istio kubeconfig: %s", err.Error())
	}

	//used to create svc and ep in istio cluster
	istioClient, err := kubernetes.NewForConfig(istiocfg)
	if err != nil {
		klog.Fatalf("Error building istio kubernetes clientset: %s", err.Error())
	}

	//used to create istio artifacts in istio cluster
	istioclientset, err := istioclientset.NewForConfig(istiocfg)
	if err != nil {
		klog.Fatalf("Error building istio kubernetes clientset: %s", err.Error())
	}

	//used to create service routes in istio cluster
	istiocapsclientset, err := clientset.NewForConfig(istiocfg)
	if err != nil {
		klog.Fatalf("Error building istio kubernetes clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)

	controller := NewController(kubeClient, exampleClient, istioClient, istioclientset, istiocapsclientset, clustername, istio,
		kubeInformerFactory.Core().V1().Endpoints(),
		kubeInformerFactory.Core().V1().Services(),
		exampleInformerFactory.Samplecontroller().V1alpha1().ServiceRoutes())

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	exampleInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&clustername, "clustername", "", "name of the cluster. required.")
	flag.StringVar(&istio_config, "istio_config", "", "Path to a kubeconfig to the istio cluster. required.")
	flag.StringVar(&istio.istio_ns, "istio_ns", "", "Namespace for artifacts in the istio cluster. required.")
	flag.StringVar(&istio.istio_suffix, "istio_suffix", "", "DNS suffix supported by the istio cluster. required.")
	flag.StringVar(&istio.istio_gateway, "istio_gateway", "", "DNS suffix supported by the istio cluster. required.")
	flag.StringVar(&istio.istio_gateway_http, "istio_gateway_http", "", "DNS suffix supported by the istio cluster. required.")
	flag.StringVar(&istio.istio_gateway_https, "istio_gateway_https", "", "DNS suffix supported by the istio cluster. required.")
}

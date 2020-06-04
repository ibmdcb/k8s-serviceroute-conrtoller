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
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	//appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	v1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclientset "istio.io/client-go/pkg/clientset/versioned"

	samplev1alpha1 "dell.com/routecontroller/pkg/apis/routecontroller/v1alpha1"
	clientset "dell.com/routecontroller/pkg/generated/clientset/versioned"
	samplescheme "dell.com/routecontroller/pkg/generated/clientset/versioned/scheme"
	informers "dell.com/routecontroller/pkg/generated/informers/externalversions/routecontroller/v1alpha1"
	listers "dell.com/routecontroller/pkg/generated/listers/routecontroller/v1alpha1"
)

const controllerAgentName = "sample-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a ServiceRoute is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a ServiceRoute fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by ServiceRoute"
	// MessageResourceSynced is the message used for an Event fired when a ServiceRoute
	// is synced successfully
	MessageResourceSynced = "ServiceRoute synced successfully"
)

type IstioInfo struct {
	istio_suffix        string
	istio_ns            string
	istio_gateway       string
	istio_gateway_http  string
	istio_gateway_https string
}

// Controller is the controller implementation for ServiceRoute resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface
	// kubeclientset is a standard kubernetes clientset
	istioclientset kubernetes.Interface
	// istioset to create istio resources on istio cluster
	istioset istioclientset.Interface
	// istiocapsset to create caps resources on istio cluster
	istiocapsset clientset.Interface

	clustername       string
	istio             IstioInfo
	endpointsLister   corelisters.EndpointsLister
	deploymentsSynced cache.InformerSynced
	servicesLister    corelisters.ServiceLister
	servicesSynced    cache.InformerSynced
	foosLister        listers.ServiceRouteLister
	foosSynced        cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	istioclientset kubernetes.Interface,
	istioset istioclientset.Interface,
	istiocapsset clientset.Interface,
	clustername string,
	istio IstioInfo,
	endpointsInformer coreinformers.EndpointsInformer,
	servicesInformer coreinformers.ServiceInformer,
	fooInformer informers.ServiceRouteInformer) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:     kubeclientset,
		sampleclientset:   sampleclientset,
		istioclientset:    istioclientset,
		istioset:          istioset,
		istiocapsset:      istiocapsset,
		clustername:       clustername,
		istio:             istio,
		endpointsLister:   endpointsInformer.Lister(),
		deploymentsSynced: endpointsInformer.Informer().HasSynced,
		servicesLister:    servicesInformer.Lister(),
		servicesSynced:    servicesInformer.Informer().HasSynced,
		foosLister:        fooInformer.Lister(),
		foosSynced:        fooInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ServiceRoutes"),
		recorder:          recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when ServiceRoute resources change
	fooInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueServiceRoute,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueServiceRoute(new)
		},
		DeleteFunc: controller.enqueueServiceRoute,
	})
	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a ServiceRoute resource will enqueue that ServiceRoute resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	endpointsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*corev1.Endpoints)
			oldDepl := old.(*corev1.Endpoints)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting ServiceRoute controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.foosSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process ServiceRoute resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// ServiceRoute resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the ServiceRoute resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	istiocaps := fmt.Sprintf("%s--%s--%s", c.clustername, namespace, name)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the ServiceRoute resource with this namespace/name
	foo, fooerr := c.foosLister.ServiceRoutes(namespace).Get(name)
	istiofoo, iferr := c.istiocapsset.SamplecontrollerV1alpha1().ServiceRoutes(c.istio.istio_ns).Get(context.TODO(), istiocaps, metav1.GetOptions{})

	routetodelete := ""
	if fooerr != nil {
		// The ServiceRoute resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(fooerr) && iferr == nil {
			routetodelete = istiofoo.Spec.RouteName
		}
	} else {
		//no err, so found routes on src
		if iferr == nil && (foo.Spec.RouteName != istiofoo.Spec.RouteName) {
			routetodelete = istiofoo.Spec.RouteName
		}
	}

	if routetodelete != "" {

		//deploymentName := foo.Spec.ServiceName
		routeName := routetodelete
		err = c.istioclientset.CoreV1().Endpoints(c.istio.istio_ns).Delete(context.TODO(), routeName, metav1.DeleteOptions{})
		err = c.istioclientset.CoreV1().Services(c.istio.istio_ns).Delete(context.TODO(), routeName, metav1.DeleteOptions{})
		err = c.istioset.NetworkingV1alpha3().VirtualServices(c.istio.istio_ns).Delete(context.TODO(), routeName, metav1.DeleteOptions{})
		err = c.istioset.NetworkingV1alpha3().DestinationRules(c.istio.istio_ns).Delete(context.TODO(), routeName, metav1.DeleteOptions{})
		gateway, err := c.istioset.NetworkingV1alpha3().Gateways(c.istio.istio_ns).Get(context.TODO(), c.istio.istio_gateway, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			klog.Infof("Can't find Gateway : %s", c.istio.istio_gateway)
		} else {
			for i := 0; i < len(gateway.Spec.Servers); i++ {
				klog.Infof("Found gateway server name : %s", gateway.Spec.Servers[i].Port.Name)
				if gateway.Spec.Servers[i].Port.Name == c.istio.istio_gateway_http || gateway.Spec.Servers[i].Port.Name == c.istio.istio_gateway_https {
					for j, host := range gateway.Spec.Servers[i].Hosts {
						if host == fmt.Sprintf("%s.%s", routeName, c.istio.istio_suffix) {
							gateway.Spec.Servers[i].Hosts[j] = gateway.Spec.Servers[i].Hosts[len(gateway.Spec.Servers[i].Hosts)-1]
							gateway.Spec.Servers[i].Hosts[len(gateway.Spec.Servers[i].Hosts)-1] = ""
							gateway.Spec.Servers[i].Hosts = gateway.Spec.Servers[i].Hosts[:len(gateway.Spec.Servers[i].Hosts)-1]
							break
						}
					}
				}
			}
			klog.Infof("Update gateway %s", gateway.Spec.Servers)
			c.istioset.NetworkingV1alpha3().Gateways(c.istio.istio_ns).Update(context.TODO(), gateway, metav1.UpdateOptions{})
		}
		c.istiocapsset.SamplecontrollerV1alpha1().ServiceRoutes(c.istio.istio_ns).Delete(context.TODO(), istiocaps, metav1.DeleteOptions{})

	}

	if fooerr != nil {
		if errors.IsNotFound(fooerr) {
			utilruntime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}
		return fooerr
	}

	deploymentName := foo.Name
	routeName := foo.Spec.RouteName

	if routeName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: route name must be specified", key))
		return nil
	}

	// Get the deployment with the name specified in ServiceRoute.spec
	ep, err := c.endpointsLister.Endpoints(foo.Namespace).Get(deploymentName)
	service, svcerr := c.servicesLister.Services(foo.Namespace).Get(deploymentName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) || errors.IsNotFound(svcerr) {
		utilruntime.HandleError(fmt.Errorf("%s: src endpoint or svc doesn't exist", deploymentName))
		return nil
	} else {

		_, err := c.istiocapsset.SamplecontrollerV1alpha1().ServiceRoutes(c.istio.istio_ns).Get(context.TODO(), istiocaps, metav1.GetOptions{})
		_, istiosvcerr := c.istioclientset.CoreV1().Services(c.istio.istio_ns).Get(context.TODO(), routeName, metav1.GetOptions{})

		//istio record doesn't exist, new
		if errors.IsNotFound(err) {

			//istio svc routename doesn't exist either, then ACTION:CREATE
			if errors.IsNotFound(istiosvcerr) {
				klog.Infof("Create caps on istio %s", istiocaps)
				_, err = c.istiocapsset.SamplecontrollerV1alpha1().ServiceRoutes(c.istio.istio_ns).Create(context.TODO(), c.newServiceRoutes(service, foo, c.clustername), metav1.CreateOptions{})
				klog.Infof("Create caps on istio : %s", err)

				klog.Infof("Create ednpoints %s", deploymentName)
				ep, err = c.istioclientset.CoreV1().Endpoints(c.istio.istio_ns).Create(context.TODO(), c.newEndpoint(ep, foo), metav1.CreateOptions{})
				klog.Infof("Create ednpoints result : %s", ep)

				klog.Infof("Create service %s", deploymentName)
				_, err = c.istioclientset.CoreV1().Services(c.istio.istio_ns).Create(context.TODO(), c.newService(service, foo), metav1.CreateOptions{})
				klog.Infof("Create service result : %s", service)

				klog.Infof("Create virutal service %s", routeName)
				_, err = c.istioset.NetworkingV1alpha3().VirtualServices(c.istio.istio_ns).Create(context.TODO(), c.newVirtualService(service, foo), metav1.CreateOptions{})
				klog.Infof("Create virtual service err : %s", err)

				klog.Infof("Create destination rule %s", routeName)
				_, err = c.istioset.NetworkingV1alpha3().DestinationRules(c.istio.istio_ns).Create(context.TODO(), c.newDestinationRule(service, foo), metav1.CreateOptions{})
				klog.Infof("Create destination rule err : %s", err)

				gateway, err := c.istioset.NetworkingV1alpha3().Gateways(c.istio.istio_ns).Get(context.TODO(), c.istio.istio_gateway, metav1.GetOptions{})
				if errors.IsNotFound(err) {
					klog.Infof("Can't find Gateway : %s", c.istio.istio_gateway)
				} else {
					for i := 0; i < len(gateway.Spec.Servers); i++ {
						klog.Infof("Found gateway server name : %s", gateway.Spec.Servers[i].Port.Name)
						if gateway.Spec.Servers[i].Port.Name == c.istio.istio_gateway_http || gateway.Spec.Servers[i].Port.Name == c.istio.istio_gateway_https {
							found := false
							for _, host := range gateway.Spec.Servers[i].Hosts {
								if host == fmt.Sprintf("%s.%s", routeName, c.istio.istio_suffix) {
									found = true
								}
							}
							if !found {
								gateway.Spec.Servers[i].Hosts = append(gateway.Spec.Servers[i].Hosts, fmt.Sprintf("%s.%s", routeName, c.istio.istio_suffix))
							}
						}
					}
					klog.Infof("Update gateway %s", gateway.Spec.Servers)
					c.istioset.NetworkingV1alpha3().Gateways(c.istio.istio_ns).Update(context.TODO(), gateway, metav1.UpdateOptions{})

					fooCopy := foo.DeepCopy()
					fooCopy.Status.FullRouteName = fmt.Sprintf("%s.%s", routeName, c.istio.istio_suffix)
					c.sampleclientset.SamplecontrollerV1alpha1().ServiceRoutes(foo.Namespace).Update(context.TODO(), fooCopy, metav1.UpdateOptions{})
				}
			} else {
				//ACTION:NO_CREATION
				fooCopy := foo.DeepCopy()
				fooCopy.Status.FullRouteName = fmt.Sprintf("%s already taken. please change routeName.", routeName)
				c.sampleclientset.SamplecontrollerV1alpha1().ServiceRoutes(foo.Namespace).Update(context.TODO(), fooCopy, metav1.UpdateOptions{})
				utilruntime.HandleError(fmt.Errorf("%s: route already exists", deploymentName))
				return nil
			}
		} else {
			//ACTION:UPDATE ENDPOINTS
			klog.Infof("Update service %s", deploymentName)
			_, err = c.istioclientset.CoreV1().Services(c.istio.istio_ns).Update(context.TODO(), c.newService(service, foo), metav1.UpdateOptions{})
			klog.Infof("update service result : %s", service)

			klog.Infof("Update ednpoints %s", deploymentName)
			ep, err = c.istioclientset.CoreV1().Endpoints(c.istio.istio_ns).Update(context.TODO(), c.newEndpoint(ep, foo), metav1.UpdateOptions{})
			klog.Infof("update ednpoints result : %s", ep)

		}
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this ServiceRoute resource, we should log
	// a warning to the event recorder and return error msg.
	//if !metav1.IsControlledBy(deployment, foo) {
	//	msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
	//	c.recorder.Event(foo, corev1.EventTypeWarning, ErrResourceExists, msg)
	//	return fmt.Errorf(msg)
	//}

	// If this number of the replicas on the ServiceRoute resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.
	//if foo.Spec.Replicas != nil && *foo.Spec.Replicas != *deployment.Spec.Replicas {
	//	klog.V(4).Infof("ServiceRoute %s replicas: %d, deployment replicas: %d", name, *foo.Spec.Replicas, *deployment.Spec.Replicas)
	//	deployment, err = c.kubeclientset.AppsV1().Deployments(foo.Namespace).Update(context.TODO(), newDeployment(foo), metav1.UpdateOptions{})
	//}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the ServiceRoute resource to reflect the
	// current state of the world
	//YZ
	//err = c.updateServiceRouteStatus(foo, deployment)
	//if err != nil {
	//	return err
	//}

	c.recorder.Event(foo, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// enqueueServiceRoute takes a ServiceRoute resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than ServiceRoute.
func (c *Controller) enqueueServiceRoute(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the ServiceRoute resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that ServiceRoute resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if object != nil {

		foo, err := c.foosLister.ServiceRoutes(object.GetNamespace()).Get(object.GetName())
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of foo '%s'", object.GetSelfLink(), object.GetName())
			return
		}

		c.enqueueServiceRoute(foo)
		return
	}
}

// newDeployment creates a new Deployment for a ServiceRoute resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ServiceRoute resource that 'owns' it.
func (c *Controller) newEndpoint(ep *corev1.Endpoints, foo *samplev1alpha1.ServiceRoute) *corev1.Endpoints {

	newep := corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      foo.Spec.RouteName,
			Namespace: c.istio.istio_ns,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{{IP: "1.2.3.4"}},
				Ports:     []corev1.EndpointPort{{Port: 80, Protocol: corev1.ProtocolTCP}},
			},
		},
	}
	newep.Subsets = ep.Subsets
	return &newep
}

// newDeployment creates a new Deployment for a ServiceRoute resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ServiceRoute resource that 'owns' it.
func (c *Controller) newService(svc *corev1.Service, foo *samplev1alpha1.ServiceRoute) *corev1.Service {

	newsvc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      foo.Spec.RouteName,
			Namespace: c.istio.istio_ns,
		},
		Spec: corev1.ServiceSpec{
			Ports: svc.Spec.Ports,
			Type:  corev1.ServiceTypeClusterIP,
		},
	}

	for i := 0; i < len(newsvc.Spec.Ports); i++ {
		newsvc.Spec.Ports[i].NodePort = 0
	}

	return &newsvc
}

// newDeployment creates a new Deployment for a ServiceRoute resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ServiceRoute resource that 'owns' it.
func (c *Controller) newVirtualService(svc *corev1.Service, foo *samplev1alpha1.ServiceRoute) *v1alpha3.VirtualService {

	newsvc := v1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      foo.Spec.RouteName,
			Namespace: c.istio.istio_ns,
		},
		Spec: networkingv1alpha3.VirtualService{
			Gateways: []string{c.istio.istio_gateway, "mesh"},
			Hosts:    []string{fmt.Sprintf("%s.%s", foo.Spec.RouteName, c.istio.istio_suffix)},
			Http: []*networkingv1alpha3.HTTPRoute{
				{
					Match: []*networkingv1alpha3.HTTPMatchRequest{
						{
							Gateways: []string{c.istio.istio_gateway, "mesh"},
						},
					},
					Route: []*networkingv1alpha3.HTTPRouteDestination{
						{
							Destination: &networkingv1alpha3.Destination{
								Host: fmt.Sprintf("%s.%s.svc.cluster.local", foo.Spec.RouteName, c.istio.istio_ns),
								Port: &networkingv1alpha3.PortSelector{
									Number: uint32(svc.Spec.Ports[0].Port),
								},
							},
						},
					},
				},
			},
		},
	}

	return &newsvc
}

// newDeployment creates a new Deployment for a ServiceRoute resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ServiceRoute resource that 'owns' it.
func (c *Controller) newDestinationRule(svc *corev1.Service, foo *samplev1alpha1.ServiceRoute) *v1alpha3.DestinationRule {

	newsvc := v1alpha3.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      foo.Spec.RouteName,
			Namespace: c.istio.istio_ns,
		},
		Spec: networkingv1alpha3.DestinationRule{
			Host: fmt.Sprintf("%s.%s.svc.cluster.local", foo.Spec.RouteName, c.istio.istio_ns),
			TrafficPolicy: &networkingv1alpha3.TrafficPolicy{
				Tls: &networkingv1alpha3.ClientTLSSettings{
					Mode: networkingv1alpha3.ClientTLSSettings_TLSmode(networkingv1alpha3.ClientTLSSettings_DISABLE),
				},
			},
		},
	}

	return &newsvc
}

// newDeployment creates a new Deployment for a ServiceRoute resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ServiceRoute resource that 'owns' it.
func (c *Controller) newServiceRoutes(svc *corev1.Service, foo *samplev1alpha1.ServiceRoute, clustername string) *samplev1alpha1.ServiceRoute {

	newsvc := samplev1alpha1.ServiceRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s--%s--%s", clustername, foo.Namespace, foo.Name),
			Namespace: c.istio.istio_ns,
		},
		Spec: samplev1alpha1.ServiceRouteSpec{
			//ServiceName: foo.Spec.ServiceName,
			RouteName: foo.Spec.RouteName,
		},
	}

	return &newsvc
}

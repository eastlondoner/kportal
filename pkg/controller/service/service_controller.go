package service

import (
	"bytes"
	"context"
	"github.com/eastlondoner/kportal/pkg/proxy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"os/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"github.com/subchen/go-log"
	"reflect"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Service Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this core.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	minikubeIp := getMinikubeIp()
	proxies := proxy.New(minikubeIp)
	proxies.RunDNS()
	proxies.RunTCPProxy()
	return &ReconcileService{
		Client:                   mgr.GetClient(),
		scheme:                   mgr.GetScheme(),
		proxy:                    proxies,
		knownServicesByNamespace: make(map[string]map[string]corev1.Service, 0),
	}
}

func getMinikubeIp() string {
	cmd := exec.Command("minikube", "ip")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		panic(err)
	}
	result := out.String()
	return strings.TrimSpace(result)

}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("service-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Service
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by Service - change this for objects you create
	// eastlondoner: We don't create any resources in this controller
	//err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
	//	IsController: true,
	//	OwnerType:    &corev1.Service{},
	//})
	//if err != nil {
	//	return err
	//}

	return nil
}

// ReconcileService reconciles a Service object
type ReconcileService struct {
	client.Client
	scheme                   *runtime.Scheme
	knownServicesByNamespace map[string]map[string]corev1.Service // Using map[string]bool to implement set[string] because this is go
	proxy                    *proxy.Proxies
}

// Reconcile reads that state of the cluster for a Service object and makes changes based on the state read
// and what is in the Service.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch
func (r *ReconcileService) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Service instance
	instance := &corev1.Service{}
	if request.Namespace == "kube-system" {
		// Leave system services alone
		return reconcile.Result{}, nil
	}
	log.Infof("Reconcile %s", request.Name)
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, allow logic below to run
		} else {
			// Error reading the object - requeue the request.
			return reconcile.Result{}, err
		}
	}

	serviceList := &corev1.ServiceList{}
	err = r.List(context.TODO(), &client.ListOptions{
		Namespace: request.Namespace,
	}, serviceList)
	if err != nil {
		panic(err)
	}

	svcSet := make(map[string]corev1.Service)
	for _, svc := range serviceList.Items {
		svcSet[svc.Name] = *svc.DeepCopy()
	}

	if knownServices, ok := r.knownServicesByNamespace[request.Namespace]; ok {
		if areServicesTheSame(knownServices, svcSet) {
			log.Info("Services are unchanged")
			// Nothing to do
			return reconcile.Result{}, nil
		}
	}
	r.knownServicesByNamespace[request.Namespace] = svcSet

	r.proxy.ReconfigureProxies(r.knownServicesByNamespace)

	return reconcile.Result{}, nil
}

func areServicesTheSame(a, b map[string]corev1.Service) bool {
	if len(a) != len(b) {
		return false
	}

	for k, va := range a {
		if vb, ok := b[k]; !ok {
			return false
		} else {
			if va.Annotations["wildcards.kportal.io"] != vb.Annotations["wildcards.kportal.io"] {
				return false
			}
			if !reflect.DeepEqual(va.Spec.Ports, vb.Spec.Ports) {
				return false
			}
		}
	}
	return true
}

package service

import (
	"testing"
	"time"

	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
var instanceKey = types.NamespacedName{Name: "foo", Namespace: "default"}

const timeout = time.Second * 5
const TEST_TIMEOUT = time.Minute

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	testCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(TEST_TIMEOUT))
	defer cancel()

	instance := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				corev1.ServicePort{Port: 8080, TargetPort: intstr.FromInt(8081)},
			},
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create the Service object and expect the Reconcile and Deployment to be created
	err = c.Create(testCtx, instance)
	// The instance object may not be a valid object because it might be missing some required fields.
	// Please modify the instance object by adding required fields and then remove the following if statement.
	if apierrors.IsInvalid(err) {
		t.Fatalf("failed to create object, got an invalid object error: %v", err)
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(testCtx, instance)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	fetchedSvc := &corev1.Service{}
	// Delete the Service and expect Reconcile to be called for Service deletion
	g.Expect(c.Delete(testCtx, instance)).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	g.Eventually(func() error { return c.Get(testCtx, instanceKey, fetchedSvc) }, timeout).
		Should(gomega.MatchError("Service \"foo\" not found"))

}

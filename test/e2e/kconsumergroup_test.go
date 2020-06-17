package e2e

import (
	goctx "context"
	"fmt"
	"testing"
	"time"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/thanhnamit/kconsumer-group-operator/pkg/apis"
	thenextappsv1alpha1 "github.com/thanhnamit/kconsumer-group-operator/pkg/apis/thenextapps/v1alpha1"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
	name                 = "kconsumer"
	image                = "thenextapps/kconsumer:latest"
	topicName            = "fast-data-topic"
)

func TestKconsumerGroup(t *testing.T) {
	kconsumerGroupList := &thenextappsv1alpha1.KconsumerGroupList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, kconsumerGroupList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	// run subtests
	t.Run("kconsumer-group", func(t *testing.T) {
		t.Run("Cluster", KconsumerGroupCluster)
		t.Run("Cluster2", KconsumerGroupCluster)
	})
}

func kconsumerGroupScaleTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}
	// create custom resource
	example := &thenextappsv1alpha1.KconsumerGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: thenextappsv1alpha1.KconsumerGroupSpec{
			MinReplicas:            1,
			AverageRecordsLagLimit: 1000,
			ConsumerSpec: thenextappsv1alpha1.ConsumerSpec{
				Image:   image,
				PodName: name,
				Topic:   topicName,
			},
		},
	}
	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err = f.Client.Create(goctx.TODO(), example, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}
	// wait for kconsumer to reach 1 replicas
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, name, 1, retryInterval, timeout)
	if err != nil {
		return err
	}

	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, example)
	if err != nil {
		return err
	}

	example.Spec.MinReplicas = 4
	err = f.Client.Update(goctx.TODO(), example)
	if err != nil {
		return err
	}

	// wait for kconsumer to reach 4 replicas
	return e2eutil.WaitForDeployment(t, f.KubeClient, namespace, name, 4, retryInterval, timeout)
}

func KconsumerGroupCluster(t *testing.T) {
	t.Parallel()
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}
	t.Log("Initialized cluster resources")
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}
	// get global framework variables
	f := framework.Global
	// wait for kconsumer-group-operator to be ready
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "kconsumer-group-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	if err = kconsumerGroupScaleTest(t, f, ctx); err != nil {
		t.Fatal(err)
	}
}

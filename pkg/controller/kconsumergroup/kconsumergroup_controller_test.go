package kconsumergroup

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	thenextappsv1alpha1 "github.com/thanhnamit/kconsumer-group-operator/pkg/apis/thenextapps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	name                = "kconsumer"
	namespace           = "default"
	replicas      int32 = 3
	lagLimit      int32 = 1000
	image               = "thenextapps/kconsumer:latest"
	containerName       = "kconsumer"
	topicName           = "fast-data-topic"
	partitionSize int32 = 5
)

func TestKconsumerGroupController(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	objMetaData := getObjectMeta()
	kgrp := getKconsumerGroup(objMetaData)
	topic := getTopicStruct(partitionSize)
	sm, hpa, pr := getChildObjs(objMetaData)

	// Objects to track in the fake client.
	objs := []runtime.Object{
		kgrp, topic,
	}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(thenextappsv1alpha1.SchemeGroupVersion, kgrp)
	s.AddKnownTypes(monitoringv1.SchemeGroupVersion, sm)
	s.AddKnownTypes(monitoringv1.SchemeGroupVersion, pr)
	s.AddKnownTypes(autoscaling.SchemeGroupVersion, hpa)
	s.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Kind:    "KafkaTopic",
		Version: "v1beta1",
	}, topic)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)

	// Create a ReconcileKconsumerGroup object with the scheme and fake client.
	r := &ReconcileKconsumerGroup{client: cl, scheme: s}

	// Mock request to simulate Reconcile() being called on an event for a watched owned resource .
	req := mockRequest(namespace, name)

	// ===== test Service to be created
	reconcileAndCheckRequeue(t, r, req)
	ser := &corev1.Service{}
	err := cl.Get(context.TODO(), req.NamespacedName, ser)
	if err != nil {
		t.Fatalf("get service: (%v)", err)
	}

	// ===== test Service Monitor to be created
	reconcileAndCheckRequeue(t, r, req)
	ksm := &monitoringv1.ServiceMonitor{}
	err = cl.Get(context.TODO(), req.NamespacedName, ksm)
	if err != nil {
		t.Fatalf("get service monitor: (%v)", err)
	}
	jobLabel := ksm.Spec.JobLabel
	if jobLabel != name {
		t.Errorf("service monitor job name (%s) is not the expected (%s)", jobLabel, name)
	}

	// ===== test Prometheus rule to be created
	reconcileAndCheckRequeue(t, r, req)
	kpr := &monitoringv1.PrometheusRule{}
	err = cl.Get(context.TODO(), req.NamespacedName, kpr)
	if err != nil {
		t.Fatalf("get prometheus rule: (%v)", err)
	}
	ruleName := kpr.Spec.Groups[0].Name
	alertName := "./kconsumer.rules"
	if ruleName != alertName {
		t.Errorf("prometheus rule name name (%s) is not the expected (%s)", ruleName, alertName)
	}

	// ===== test HPA to be created
	reconcileAndCheckRequeue(t, r, req)
	khpa := &autoscaling.HorizontalPodAutoscaler{}
	err = cl.Get(context.TODO(), req.NamespacedName, khpa)
	if err != nil {
		t.Fatalf("get hpa: (%v)", err)
	}

	hpaMinReps := khpa.Spec.MinReplicas
	if *hpaMinReps != replicas {
		t.Errorf("hpa min replicas (%d) is not the expected (%d)", hpaMinReps, replicas)
	}

	hpaMaxReps := khpa.Spec.MaxReplicas
	if hpaMaxReps != partitionSize {
		t.Errorf("hpa max replicas (%d) is not the expected (%d)", hpaMaxReps, partitionSize)
	}

	// ===== test Deployment to be created
	req = mockRequest(namespace, name)
	reconcileAndCheckRequeue(t, r, req)
	dep := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), req.NamespacedName, dep)
	if err != nil {
		t.Fatalf("get deployment: (%v)", err)
	}

	dsize := *dep.Spec.Replicas
	if dsize != replicas {
		t.Errorf("dep size (%d) is not the expected size (%d)", dsize, replicas)
	}

	// ===== test Kconsumer Status
	podLabels := appLabels(name)
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Labels:    podLabels,
		},
	}
	podNames := make([]string, 3)
	for i := 0; i < 3; i++ {
		pod.ObjectMeta.Name = name + "." + strconv.Itoa(rand.Int())
		podNames[i] = pod.ObjectMeta.Name
		if err = cl.Create(context.TODO(), pod.DeepCopy()); err != nil {
			t.Fatalf("create pod %d: (%v)", i, err)
		}
	}

	// Reconcile again so Reconcile() checks pods and updates the KconsumerGroup
	// resources' Status.
	reconcileAndCheckEmptyResult(t, r, req)

	// Get the updated KconsumerGroup object.
	kgrp = &thenextappsv1alpha1.KconsumerGroup{}
	err = r.client.Get(context.TODO(), req.NamespacedName, kgrp)
	if err != nil {
		t.Errorf("get Kconsumergroup: (%v)", err)
	}

	// Ensure Reconcile() updated the KConsumerGroup's Status as expected.
	pods := kgrp.Status.ActivePods
	if !reflect.DeepEqual(podNames, pods) {
		t.Errorf("pod names %v did not match expected %v", pods, podNames)
	}

	// ===== test final status
	msg := kgrp.Status.Message
	finalMsg := "Message: Reconciliation completed"
	if !strings.HasPrefix(msg, finalMsg) {
		t.Errorf("final message %s did not match expected %s", msg, finalMsg)
	}
}

func reconcileAndCheckRequeue(t *testing.T, r *ReconcileKconsumerGroup, req reconcile.Request) {
	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Check the result of reconciliation to make sure it has the desired state.
	if !res.Requeue {
		t.Error("reconcile did not requeue request as expected")
	}
}

func reconcileAndCheckEmptyResult(t *testing.T, r *ReconcileKconsumerGroup, req reconcile.Request) {
	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	if res != (reconcile.Result{}) {
		t.Error("reconcile did not return an empty Result")
	}
}

func mockRequest(namespace string, name string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func getChildObjs(objMetaData metav1.ObjectMeta) (*monitoringv1.ServiceMonitor, *autoscaling.HorizontalPodAutoscaler, *monitoringv1.PrometheusRule) {
	sm := &monitoringv1.ServiceMonitor{
		ObjectMeta: objMetaData,
	}

	hpa := &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: objMetaData,
	}

	pr := &monitoringv1.PrometheusRule{
		ObjectMeta: objMetaData,
	}
	return sm, hpa, pr
}

func getObjectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
}

// TODO: may be error here
func getTopicStruct(partitions int32) *unstructured.Unstructured {
	topic := &unstructured.Unstructured{}
	topic.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "kafka.strimzi.io/v1beta1",
		"kind":       "KafkaTopic",
		"metadata": map[string]interface{}{
			"name":      topicName,
			"namespace": namespace,
			"labels": map[string]interface{}{
				"strimzi.io/cluster": "my-cluster",
			},
		},
		"spec": map[string]interface{}{
			"partitions": fmt.Sprintf("%d", partitions),
			"replicas":   "1",
		},
	})
	return topic
}

func getKconsumerGroup(objectMetaData metav1.ObjectMeta) *thenextappsv1alpha1.KconsumerGroup {
	return &thenextappsv1alpha1.KconsumerGroup{
		ObjectMeta: objectMetaData,
		Spec: thenextappsv1alpha1.KconsumerGroupSpec{
			MinReplicas:            replicas,
			AverageRecordsLagLimit: lagLimit,
			ConsumerSpec: thenextappsv1alpha1.ConsumerSpec{
				Image:   image,
				PodName: containerName,
				Topic:   topicName,
			},
		},
	}
}

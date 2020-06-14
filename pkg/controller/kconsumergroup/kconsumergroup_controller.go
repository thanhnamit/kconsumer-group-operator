package kconsumergroup

import (
	"context"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	thenextappsv1alpha1 "github.com/thanhnamit/kconsumer-group-operator/pkg/apis/thenextapps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "controller_kconsumergroup"
)

var log = logf.Log.WithName(controllerName)

// Add creates a new KconsumerGroup Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileKconsumerGroup{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource KconsumerGroup
	err = c.Watch(&source.Kind{Type: &thenextappsv1alpha1.KconsumerGroup{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Service and requeue the owner KconsumerGroup
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &thenextappsv1alpha1.KconsumerGroup{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource HorizontalAutoScaler
	err = c.Watch(&source.Kind{Type: &autoscaling.HorizontalPodAutoscaler{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &thenextappsv1alpha1.KconsumerGroup{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource ServiceMonitor
	err = c.Watch(&source.Kind{Type: &monitoringv1.ServiceMonitor{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &thenextappsv1alpha1.KconsumerGroup{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource PrometheusRule
	err = c.Watch(&source.Kind{Type: &monitoringv1.PrometheusRule{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &thenextappsv1alpha1.KconsumerGroup{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to KafkaTopic (not created by kconsumer)
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "KafkaTopic",
		Group:   "kafka.strimzi.io",
		Version: "v1beta1",
	})
	u.SetName("fast-data-topic")
	err = c.Watch(&source.Kind{Type: u}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileKconsumerGroup implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileKconsumerGroup{}

// ReconcileKconsumerGroup reconciles a KconsumerGroup object
type ReconcileKconsumerGroup struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a KconsumerGroup object and makes changes based on the state read
// and what is in the KconsumerGroup.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileKconsumerGroup) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling KconsumerGroup")

	// Fetch the KconsumerGroup instance
	instance := &thenextappsv1alpha1.KconsumerGroup{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Define a new Pod object
	pod := newPodForCR(instance)

	// Set KconsumerGroup instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	found := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		err = r.client.Create(context.TODO(), pod)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Pod already exists - don't requeue
	reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcileKconsumerGroup) manageService(kgrp *thenextappsv1alpha1.KConsumerGroup, reqLogger logr.Logger) (*reconcile.Result, error) {
	service := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: kgrp.Name, Namespace: kgrp.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		err := r.createKConsumerService(kgrp, service)
		if err != nil {
			reqLogger.Error(err, "Error getting consumer service")
			return &reconcile.Result{}, err
		}
		reqLogger.Info("Adding new Kconsumer service: ", service.Namespace, ":", service.Name)
		err := r.client.Create(context.TODO(), service)
		if err != nil {
			reqLogger.Error(err, "Errors creating Kconsumer service: ", service.Namespace, ":", service.Name)
			return &reconcile.Result{}, err
		}
	}
}

func (r *ReconcileKconsumerGroup) manageDeployment(kgrp *thenextappsv1alpha1.KconsumerGroup, reqLogger logr.Logger) (*reconcile.Result, error) {
	deployment := &appsv1.Deployment{}
	err := r.client.Get(Context.TODO(), types.NamespacedName{Name: kgrp.Name, Namespace: kgrp.Namespace}, deployment)
	if err != nil && errors.IsNotFound(err) {
		consumerDeployment, err := r.createKConsumerDeployment(kgrp)
		if err != nil {
			reqLogger.Error(err, "Errors getting consumer deployment")
		}
		reqLogger.Info("Adding new Kconsumer deployment: ", consumerDeployment.Namespace, ":", consumerDeployment.Name)
		err := r.client.Create(context.TODO(), consumerDeployment)
		if err != nil {
			reqLogger.Error(err, "Errors creating Kconsumer deployment: ", consumerDeployment.Namespace, ":", consumerDeployment.Name)
			return &reconcile.Result{}, err
		}
		return &reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Kconsumer deployment")
		return &reconcile.Result{}, err
	}
	return nil, nil
}

func (r *ReconcileKconsumerGroup) createKConsumerDeployment(kgrp *thenextappsv1alpha1.KconsumerGroup) (*appsv1.Deployment, error) {
	var replicas int32 
	replicas = kgrp.Spec.MinReplicas
	labels := map[string]string {
		"app": kgrp.Name
	}
	dp := &appsv1.Deployment {
		ObjectMeta: metav1.ObjectMeta{
			Name: kgrp.Name,
			Namespace: kgrp.Namespace,
			Labels: labels,
		},
		Spec: &appsv1.DeploymentSpec {
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: kgrp.Name,
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: kgrp.Name,
							Image: kgrp.Spec.ConsumerSpec.Image,
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8085,
									Name: "metrics",
								}
							}
						}
					}
				}
			}
		},	
	}
}

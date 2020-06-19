package kconsumergroup

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	thenextappsv1alpha1 "github.com/thanhnamit/kconsumer-group-operator/pkg/apis/thenextapps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	consumerName   = "kconsumer"
	controllerName = "controller_kconsumergroup"
	kafkaTopic     = "fast-data-topic"
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

	u.SetName(kafkaTopic)
	err = c.Watch(&source.Kind{Type: u}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileKconsumerGroup{}

// ReconcileKconsumerGroup reconciles a KconsumerGroup object
type ReconcileKconsumerGroup struct {
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a KconsumerGroup object and makes changes based on the state read
// and what is in the KconsumerGroup.Spec as well as Resources not owned by the owner
func (r *ReconcileKconsumerGroup) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling KconsumerGroup")
	kgrp := &thenextappsv1alpha1.KconsumerGroup{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: request.Namespace,
		Name:      consumerName,
	}, kgrp)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Manage Service
	reconcileResult, err := r.reconcileService(kgrp)
	if err != nil {
		r.updateStatus(kgrp, "Error reconciling Service", false, err)
		return *reconcileResult, err
	} else if err == nil && reconcileResult != nil {
		return *reconcileResult, nil
	}

	// Manage ServiceMonitor
	reconcileResult, err = r.reconcileServiceMonitor(kgrp)
	if err != nil {
		r.updateStatus(kgrp, "Error reconciling Service Monitor", false, err)
		return *reconcileResult, err
	} else if err == nil && reconcileResult != nil {
		return *reconcileResult, nil
	}

	// Manage PrometheusRule
	reconcileResult, err = r.reconcilePrometheusRule(kgrp)
	if err != nil {
		r.updateStatus(kgrp, "Error reconciling Prometheus Rule", false, err)
		return *reconcileResult, err
	} else if err == nil && reconcileResult != nil {
		return *reconcileResult, nil
	}

	// Manage HPA
	reconcileResult, err = r.reconcileHPA(kgrp)
	if err != nil {
		r.updateStatus(kgrp, "Error reconciling HPA", false, err)
		return *reconcileResult, err
	} else if err == nil && reconcileResult != nil {
		return *reconcileResult, nil
	}

	// Manage Kafka topic changes
	reconcileResult, err = r.reconcileTopicChange(kgrp, request)
	if err != nil {
		reqLogger.Error(err, "Error with reconciling topic")
		r.updateStatus(kgrp, "Error reconciling Kconsumer group for topic changes", false, err)
		return *reconcileResult, err
	} else if err == nil && reconcileResult != nil {
		reqLogger.Error(err, "No Error with reconciling topic")
		return *reconcileResult, nil
	}

	// Manage Deployment
	reconcileResult, err = r.reconcileDeployment(kgrp)
	if err != nil {
		r.updateStatus(kgrp, "Error reconciling Deployment", false, err)
		return *reconcileResult, err
	} else if err == nil && reconcileResult != nil {
		return *reconcileResult, nil
	}

	log.Info("Update reconciliation status")
	reconcileResult, err = r.updateStatus(kgrp, "Reconciliation completed", true, nil)
	return *reconcileResult, err
}

func (r *ReconcileKconsumerGroup) reconcileTopicChange(kgrp *thenextappsv1alpha1.KconsumerGroup, request reconcile.Request) (*reconcile.Result, error) {
	if request.Name != kafkaTopic {
		return nil, nil
	}
	log.Info("Reconciling HPA topic changes")
	return r.reconcileHPA(kgrp)
}

func (r *ReconcileKconsumerGroup) reconcileService(kgrp *thenextappsv1alpha1.KconsumerGroup) (*reconcile.Result, error) {
	service := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: kgrp.Name, Namespace: kgrp.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Kconsumer service")
		service, err := r.createService(kgrp)
		err = r.createObj(service)
		return r.handleCreationResult("Errors creating Kconsumer service", err)
	} else if err != nil {
		return r.handleFetchingErr("Failed to get Kconsumer service", err)
	} else {
		log.Info("Not handling KConsumer service update")
	}
	return nil, nil
}

func (r *ReconcileKconsumerGroup) createService(kgrp *thenextappsv1alpha1.KconsumerGroup) (*corev1.Service, error) {
	labels := appLabels(kgrp.Name)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consumerName,
			Namespace: kgrp.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port:       8085,
					Name:       "metrics",
					TargetPort: intstr.FromInt(8085),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: labels,
		},
	}

	return service, r.setOwnerReference(kgrp, service)
}

func (r *ReconcileKconsumerGroup) reconcileServiceMonitor(kgrp *thenextappsv1alpha1.KconsumerGroup) (*reconcile.Result, error) {
	sm := &monitoringv1.ServiceMonitor{}
	err := r.getObj(kgrp, sm)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Kconsumer service monitor")
		sm, err := r.createServiceMonitor(kgrp)
		err = r.createObj(sm)
		return r.handleCreationResult("Errors creating Kconsumer service monitor", err)
	} else if err != nil {
		return r.handleFetchingErr("Failed to get Kconsumer service monitoring", err)
	} else {
		log.Info("Not handling Service Monitor update")
		return nil, nil
	}
}

func (r *ReconcileKconsumerGroup) createServiceMonitor(kgrp *thenextappsv1alpha1.KconsumerGroup) (*monitoringv1.ServiceMonitor, error) {
	labels := appLabels(kgrp.Name)
	sm := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kgrp.Name,
			Namespace: kgrp.Namespace,
			Labels:    labels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			JobLabel: kgrp.Name,
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{
					kgrp.Namespace,
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:        "metrics",
					Path:        "/actuator/prometheus",
					Scheme:      "http",
					Interval:    "5s",
					HonorLabels: true,
				},
			},
		},
	}
	return sm, r.setOwnerReference(kgrp, sm)
}

func (r *ReconcileKconsumerGroup) reconcileDeployment(kgrp *thenextappsv1alpha1.KconsumerGroup) (*reconcile.Result, error) {
	deployment := &appsv1.Deployment{}
	err := r.getObj(kgrp, deployment)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Kconsumer deployment")
		deployment, err := r.createDeployment(kgrp)
		err = r.createObj(deployment)
		return r.handleCreationResult("Errors creating Kconsumer deployment", err)
	} else if err != nil {
		return r.handleFetchingErr("Failed to get Kconsumer deployment", err)
	} else {
		updateRequired, err := r.updateDeployment(kgrp, deployment)
		if updateRequired {
			log.Info("Updating Kconsumer deployment")
			err = r.updateObj(deployment)
			return r.handleUpdateResult("Failed to update Kconsumer Deployment", err)
		}
		return nil, nil
	}
}

func (r *ReconcileKconsumerGroup) createDeployment(kgrp *thenextappsv1alpha1.KconsumerGroup) (*appsv1.Deployment, error) {
	var replicas int32
	replicas = kgrp.Spec.MinReplicas
	labels := appLabels(kgrp.Name)
	dp := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kgrp.Name,
			Namespace: kgrp.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   kgrp.Name,
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            kgrp.Name,
							Image:           kgrp.Spec.ConsumerSpec.Image,
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8085,
									Name:          "metrics",
								},
							},
						},
					},
				},
			},
		},
	}
	return dp, r.setOwnerReference(kgrp, dp)
}

// updateDeployment lets HPA control the deployment replicas
func (r *ReconcileKconsumerGroup) updateDeployment(kgrp *thenextappsv1alpha1.KconsumerGroup, dp *appsv1.Deployment) (bool, error) {
	if dp.Spec.Template.Spec.Containers[0].Image != kgrp.Spec.ConsumerSpec.Image {
		dp.Spec.Template.Spec.Containers[0].Image = kgrp.Spec.ConsumerSpec.Image
		return true, r.setOwnerReference(kgrp, dp)
	}
	return false, r.setOwnerReference(kgrp, dp)
}

func (r *ReconcileKconsumerGroup) reconcilePrometheusRule(kgrp *thenextappsv1alpha1.KconsumerGroup) (*reconcile.Result, error) {
	pr := &monitoringv1.PrometheusRule{}
	err := r.getObj(kgrp, pr)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Kconsumer Prometheus Rule")
		pr, err := r.createPrometheusRule(kgrp)
		err = r.createObj(pr)
		return r.handleCreationResult("Errors creating Kconsumer Prometheus Rule", err)
	} else if err != nil {
		return r.handleFetchingErr("Failed to get Kconsumer Prometheus Rule", err)
	} else {
		log.Info("Not handling Prometheus Rule update")
		return nil, nil
	}
}

func (r *ReconcileKconsumerGroup) createPrometheusRule(kgrp *thenextappsv1alpha1.KconsumerGroup) (*monitoringv1.PrometheusRule, error) {
	topicName := kgrp.Spec.ConsumerSpec.Topic
	clientID := kgrp.Name + "-app-0"
	limit := kgrp.Spec.AverageRecordsLagLimit
	expr := fmt.Sprintf("sum(kafka_consumer_fetch_manager_records_lag{topic=\"%s\",client_id=\"%s\"}) > %d", topicName, clientID, limit)

	// Note: has to be created in same namespace as operator
	pr := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kgrp.Name,
			Namespace: kgrp.Namespace,
			Labels: map[string]string{
				"prometheus": "k8s",
				"role":       "alert-rules",
			},
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name: "./kconsumer.rules",
					Rules: []monitoringv1.Rule{
						{
							Alert: "RecordsLagBreached-" + kgrp.Name,
							Annotations: map[string]string{
								"message": "Kafka consumers is running behind for {{ $value }} messages",
							},
							Expr: intstr.FromString(expr),
							For:  "1m",
							Labels: map[string]string{
								"severity": "warning",
							},
						},
					},
				},
			},
		},
	}
	return pr, r.setOwnerReference(kgrp, pr)
}

func (r *ReconcileKconsumerGroup) reconcileHPA(kgrp *thenextappsv1alpha1.KconsumerGroup) (*reconcile.Result, error) {
	hpa := &autoscaling.HorizontalPodAutoscaler{}
	err := r.getObj(kgrp, hpa)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Kconsumer HPA")
		hpa, err := r.createHPA(kgrp)
		err = r.createObj(hpa)
		return r.handleCreationResult("Errors creating Kconsumer HPA", err)
	} else if err != nil {
		return r.handleFetchingErr("Failed to get Kconsumer HPA", err)
	} else {
		updateRequired, err := r.updateHPA(kgrp, hpa)
		if updateRequired {
			log.Info("Updating Kconsumer HPA")
			err = r.updateObj(hpa)
			return r.handleUpdateResult("Failed to update Kconsumer HPA", err)
		}
		return nil, nil
	}
}

func (r *ReconcileKconsumerGroup) createHPA(kgrp *thenextappsv1alpha1.KconsumerGroup) (*autoscaling.HorizontalPodAutoscaler, error) {
	partitions, _ := r.getTopicPartition(kgrp)
	var limit string
	limit = strconv.Itoa(int(kgrp.Spec.AverageRecordsLagLimit))
	hpa := &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kgrp.Name,
			Namespace: kgrp.Namespace,
		},
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscaling.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       kgrp.Name,
				APIVersion: "apps/v1",
			},
			MinReplicas: &kgrp.Spec.MinReplicas,
			MaxReplicas: int32(partitions),
			Metrics: []autoscaling.MetricSpec{
				{
					Type: autoscaling.PodsMetricSourceType,
					Pods: &autoscaling.PodsMetricSource{
						MetricName:         "kafka_consumer_fetch_manager_records_lag_max",
						TargetAverageValue: resource.MustParse(limit),
					},
				},
			},
		},
	}
	return hpa, r.setOwnerReference(kgrp, hpa)
}

func (r *ReconcileKconsumerGroup) updateHPA(kgrp *thenextappsv1alpha1.KconsumerGroup, hpa *autoscaling.HorizontalPodAutoscaler) (bool, error) {
	partitions, _ := r.getTopicPartition(kgrp)
	if hpa.Spec.MaxReplicas != int32(partitions) {
		hpa.Spec.MaxReplicas = int32(partitions)
		return true, r.setOwnerReference(kgrp, hpa)
	}
	return false, r.setOwnerReference(kgrp, hpa)
}

func (r *ReconcileKconsumerGroup) getTopicStruct(kgrp *thenextappsv1alpha1.KconsumerGroup) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Kind:    "KafkaTopic",
		Version: "v1beta1",
	})

	u.SetName(kgrp.Spec.ConsumerSpec.Topic)
	u.SetNamespace(kgrp.Namespace)
	return u
}

func (r *ReconcileKconsumerGroup) getTopicPartition(kgrp *thenextappsv1alpha1.KconsumerGroup) (int64, error) {
	var partitions int64
	var partitionsStr string
	u := r.getTopicStruct(kgrp)
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: u.GetNamespace(),
		Name:      u.GetName(),
	}, u)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Topic not found: " + u.GetName())
		partitions = 1
		return partitions, nil
	} else if err != nil {
		log.Info("Unable to get topic")
		partitions = 1
		return partitions, nil
	}
	log.Info("Found topic: " + kgrp.Spec.ConsumerSpec.Topic)
	partitions, found, err := unstructured.NestedInt64(u.Object, "spec", "partitions")
	if err != nil || found == false {
		// attempt again with string type
		partitionsStr, found, err = unstructured.NestedString(u.Object, "spec", "partitions")
		if err != nil || found == false {
			log.Error(err, "Problem getting topic partitions")
			log.Info("Set default partitions to 1")
			partitions = 1
			return partitions, nil
		}
		partitions, err := strconv.ParseInt(partitionsStr, 10, 64)
		if err != nil {
			partitions = 1
		}
		return partitions, nil
	}
	return partitions, nil
}

func (r *ReconcileKconsumerGroup) getObj(kgrp *thenextappsv1alpha1.KconsumerGroup, obj runtime.Object) error {
	return r.client.Get(context.TODO(), types.NamespacedName{Name: kgrp.Name, Namespace: kgrp.Namespace}, obj)
}

func (r *ReconcileKconsumerGroup) createObj(obj runtime.Object) error {
	return r.client.Create(context.TODO(), obj)
}

func (r *ReconcileKconsumerGroup) updateObj(obj runtime.Object) error {
	return r.client.Update(context.TODO(), obj)
}

func (r *ReconcileKconsumerGroup) setOwnerReference(kgrp *thenextappsv1alpha1.KconsumerGroup, obj metav1.Object) error {
	if err := controllerutil.SetControllerReference(kgrp, obj, r.scheme); err != nil {
		log.Error(err, "Error set controller reference for object")
		return err
	}
	return nil
}

func (r *ReconcileKconsumerGroup) updateStatus(kgrp *thenextappsv1alpha1.KconsumerGroup, message string, podStatus bool, err error) (*reconcile.Result, error) {
	kgrp.Status.Message = fmt.Sprintf("Message: %s, Error: %v", message, err)

	if podStatus == true {
		pods := &corev1.PodList{}
		opts := []client.ListOption{
			client.InNamespace(kgrp.Namespace),
			client.MatchingLabels(appLabels(kgrp.Name)),
		}
		if err := r.client.List(context.TODO(), pods, opts...); err != nil {
			log.Error(err, "Failed to fetch pods", kgrp.Namespace, ":", kgrp.Name)
			return &reconcile.Result{}, err
		}
		podNames := getPodNames(pods.Items)
		if !reflect.DeepEqual(podNames, kgrp.Status.ActivePods) {
			kgrp.Status.ActivePods = podNames
		}
	}

	if err := r.client.Status().Update(context.TODO(), kgrp); err != nil {
		log.Error(err, "Failed to update resource status")
		return &reconcile.Result{}, err
	}
	return &reconcile.Result{}, nil
}

func (r *ReconcileKconsumerGroup) handleCreationResult(msg string, err error) (*reconcile.Result, error) {
	if err != nil {
		log.Error(err, msg)
		return &reconcile.Result{}, err
	}
	return &reconcile.Result{Requeue: true}, nil
}

func (r *ReconcileKconsumerGroup) handleUpdateResult(msg string, err error) (*reconcile.Result, error) {
	if err != nil {
		log.Error(err, msg)
		return &reconcile.Result{}, err
	}
	return &reconcile.Result{Requeue: true}, nil
}

func (r *ReconcileKconsumerGroup) handleFetchingErr(msg string, err error) (*reconcile.Result, error) {
	log.Error(err, msg)
	return &reconcile.Result{}, err
}

func appLabels(name string) map[string]string {
	return map[string]string{"app": name}
}

func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

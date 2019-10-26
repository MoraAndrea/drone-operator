package dronefederateddeployment

import (
	"context"
	"drone-operator/drone-operator/pkg/controller/common/configuration"
	"drone-operator/drone-operator/pkg/controller/common/messaging"
	"encoding/json"
	appsv1 "k8s.io/api/apps/v1"
	"time"

	dronev1alpha1 "drone-operator/drone-operator/pkg/apis/drone/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_dronefederateddeployment")

var configurationEnv *configuration.ConfigType

var rabbit *messaging.RabbitMq

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new DroneFederatedDeployment Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDroneFederatedDeployment{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {

	// Load and create configurationEnv
	configurationEnv = configuration.Config()
	rabbit = messaging.InitRabbitMq(configurationEnv)
	rabbit.ConsumeMessage(configurationEnv.RabbitConf.QueueAdvertisementCtrl)
	rabbit.ConsumeMessage(configurationEnv.RabbitConf.QueueResult)

	// Create a new controller
	c, err := controller.New("dronefederateddeployment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource DroneFederatedDeployment
	err = c.Watch(&source.Kind{Type: &dronev1alpha1.DroneFederatedDeployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner DroneFederatedDeployment
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &dronev1alpha1.DroneFederatedDeployment{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileDroneFederatedDeployment implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileDroneFederatedDeployment{}

// ReconcileDroneFederatedDeployment reconciles a DroneFederatedDeployment object
type ReconcileDroneFederatedDeployment struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a DroneFederatedDeployment object and makes changes based on the state read
// and what is in the DroneFederatedDeployment.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileDroneFederatedDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling DroneFederatedDeployment")

	// Fetch the DroneFederatedDeployment instance
	instance := &dronev1alpha1.DroneFederatedDeployment{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after
			// reconcile request—return and don't requeue:
			return reconcile.Result{}, nil
		}
		// Error reading the object—requeue the request:
		return reconcile.Result{}, err
	}
	// If no phase set, default to pending (the initial phase):
	if instance.Status.Phase == "" {
		instance.Status.Phase = dronev1alpha1.PhasePending
	}
	// Now let's make the main case distinction: implementing
	// the state diagram PENDING -> RUNNING -> DONE
	switch instance.Status.Phase {
	case dronev1alpha1.PhasePending:
		reqLogger.Info("Phase: PENDING")
		// As long as we haven't executed the command yet, we need to check if
		// it's already time to act:
		reqLogger.Info("Checking schedule", "Target", instance.Spec.Schedule)
		// Check if it's already time to execute the command with a tolerance
		// of 2 seconds:
		d, err := timeUntilSchedule(instance.Spec.Schedule)
		if err != nil {
			reqLogger.Error(err, "Schedule parsing failure")
			// Error reading the schedule. Wait until it is fixed.
			return reconcile.Result{}, err
		}

		if d > 0 {
			// Not yet time to execute the command, wait until the scheduled time
			return reconcile.Result{RequeueAfter: d}, nil
		}
		reqLogger.Info("It's time to deploy!")
		instance.Status.Phase = dronev1alpha1.PhaseRunning
	case dronev1alpha1.PhaseRunning:
		reqLogger.Info("Phase: RUNNING")

		// CHIAMARE DRONE..... DALLA SOLUZIONE CAPIRE COSA FARE
		message := createAdvMessage(instance)
		rabbit.PublishMessage(message, configurationEnv.RabbitConf.QueueAdvertisement, false)

		pod := newPodForCR(instance)
		deploy := newDeployForCR(instance)

		// Set DroneService instance as the owner and controller
		err := controllerutil.SetControllerReference(instance, deploy, r.scheme)
		if err != nil {
			// requeue with error
			return reconcile.Result{}, err
		}

		// Check if this Pod already exists
		found := &corev1.Pod{}
		nsName := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}
		err = r.client.Get(context.TODO(), nsName, found)
		// Try to see if the pod already exists and if not
		// (which we expect) then create a one-shot pod as per spec:
		if err != nil && errors.IsNotFound(err) {
			err = r.client.Create(context.TODO(), pod)
			if err != nil {
				// requeue with error
				return reconcile.Result{}, err
			}
			// Pod created successfully - don't requeue
			reqLogger.Info("Pod launched", "name", pod.Name)
		} else if err != nil {
			// requeue with error
			return reconcile.Result{}, err
		} else if found.Status.Phase == corev1.PodFailed ||
			found.Status.Phase == corev1.PodSucceeded {
			reqLogger.Info("Container terminated", "reason", found.Status.Reason, "message", found.Status.Message)
			instance.Status.Phase = dronev1alpha1.PhaseDone
		} else {
			// Don't requeue because it will happen automatically when the
			// pod status changes.
			return reconcile.Result{}, nil
		}
	case dronev1alpha1.PhaseDone:
		reqLogger.Info("Phase: DONE")
		return reconcile.Result{}, nil
	default:
		reqLogger.Info("NOP")
		return reconcile.Result{}, nil
	}
	// Update the DroneService instance, setting the status to the respective phase:
	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	// Don't requeue. We should be reconcile because either the pod
	// or the CR changes.
	return reconcile.Result{}, nil
}

/*
func createAdvMessage(cr *dronev1alpha1.DroneFederatedDeployment) string{

	var components []messaging.Component

	for _, c := range cr.Spec.Components {
		resources := messaging.NewResources(c.Function.Resources.Memory, c.Function.Resources.Cpu)

		function := messaging.NewFunction(c.Function.Image, *resources)

		component := messaging.NewComponent(c.Name, *function, nil, c.BootDependencies, c.NodesBlacklist, c.NodesWhitelist)
		components = append(components, *component)
	}

	// Create new message
	message := messaging.NewAdvertisementMessage(cr.Spec.AppName, cr.Spec.BaseNode, cr.Spec.Type, components)
	log.Info(" Created Message %s", message)

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Error(err, "Error during marshal message...")
	}
	log.Info(" Json Message: ", string(jsonData))
	return string(jsonData)
}
*/

func createAdvMessage(cr *dronev1alpha1.DroneFederatedDeployment) string {
	var components []messaging.Component

	for _, c := range cr.Spec.Template.Spec.Template.Spec.Containers {
		resources := messaging.NewResources(float64(c.Resources.Limits.Memory().Value()), float64(c.Resources.Limits.Cpu().Value()))

		function := messaging.NewFunction(c.Image, *resources)

		bootDependencies := make([]string, 0)
		nodeBlacklist := make([]string, 0)
		var nodeWhitelist []string

		component := messaging.NewComponent(c.Name, *function, nil, bootDependencies, nodeBlacklist, nodeWhitelist)
		components = append(components, *component)
	}

	// Create new message
	message := messaging.NewAdvertisementMessage(cr.Name, configurationEnv.Kubernetes.ClusterName, messaging.ADD, components)
	// log.Info(" Created Message %s", message)

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Error(err, "Error during marshal message...")
	}
	// log.Info(" Json Message: ", string(jsonData))
	return string(jsonData)
	return string(jsonData)
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *dronev1alpha1.DroneFederatedDeployment) *corev1.Pod {

	log.Info("New pod create.....")
	labels := map[string]string{
		"app": cr.Name,
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  cr.Spec.Template.Spec.Template.Spec.Containers[0].Name,
					Image: cr.Spec.Template.Spec.Template.Spec.Containers[0].Image,
					//Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}

// newDeployForCR returns a deploy with the same name/namespace as the cr
func newDeployForCR(cr *dronev1alpha1.DroneFederatedDeployment) *appsv1.Deployment {

	log.Info("New deploy create.....")

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: cr.Name,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": cr.Spec.Template.Spec.Selector.MatchLabels["app"],
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": cr.Spec.Template.Spec.Template.Labels["app"],
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  cr.Spec.Template.Spec.Template.Spec.Containers[0].Name,
							Image: cr.Spec.Template.Spec.Template.Spec.Containers[0].Image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
}

func timeUntilSchedule(schedule string) (time.Duration, error) {
	now := time.Now().UTC()
	layout := "2006-01-02T15:04:05Z"
	s, err := time.Parse(layout, schedule)
	if err != nil {
		return time.Duration(0), err
	}
	return s.Sub(now), nil
}

package dronefederateddeployment

import (
	"context"
	"drone-operator/drone-operator/pkg/controller/common/configuration"
	"drone-operator/drone-operator/pkg/controller/common/messaging"
	"encoding/json"
	"fmt"
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

// Add creates a new DroneFederatedDeployment Controller and adds it to the Manager. The Manager will set fields on the Controller and Start it when the Manager is Started.
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
	rabbit.ConsumeMessage(configurationEnv.RabbitConf.QueueAdvertisementCtrl, printCallback)
	rabbit.ConsumeMessage(configurationEnv.RabbitConf.QueueResult, resultCallback)

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

	// Watch for changes to secondary resource Pods and requeue the owner DroneFederatedDeployment
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &dronev1alpha1.DroneFederatedDeployment{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployments and requeue the owner DroneFederatedDeployment
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &dronev1alpha1.DroneFederatedDeployment{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Services and requeue the owner DroneFederatedDeployment
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &dronev1alpha1.DroneFederatedDeployment{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Config-map and requeue the owner DroneFederatedDeployment
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
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
	// This client, initialized using mgr.Client() above, is a split client that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a DroneFederatedDeployment object and makes changes based on the state read
// and what is in the DroneFederatedDeployment.Spec
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
			// Request object not found, could have been deleted after reconcile request—return and don't requeue:
			return reconcile.Result{}, nil
		}
		// Error reading the object—requeue the request:
		return reconcile.Result{}, err
	}

	// If no phase set, default to pending (the initial phase):
	if instance.Status.Phase == "" {
		instance.Status.Phase = dronev1alpha1.PhasePending
	}

	// the state diagram PENDING -> RUNNING -> DONE
	switch instance.Status.Phase {
	case dronev1alpha1.PhasePending:
		reqLogger.Info("Phase: PENDING")

		reqLogger.Info("It's time to deploy!")
		instance.Status.Phase = dronev1alpha1.PhaseRunning
	case dronev1alpha1.PhaseRunning:
		reqLogger.Info("Phase: RUNNING")

		// DRONE Agreement, send message Advertisement
		message := createAdvMessage(instance)
		rabbit.PublishMessage(message, configurationEnv.RabbitConf.QueueAdvertisement, false)

		//pod := newPodForCR(instance)
		deploy := newDeployForCR(instance)
		jsonData, err1 := json.Marshal(deploy)
		if err1 != nil {
			log.Error(err1, "Error during marshal message...")
		}
		fmt.Println(string(jsonData))

		// Set DroneService instance as the owner and controller
		err := controllerutil.SetControllerReference(instance, deploy, r.scheme)
		if err != nil {
			// requeue with error
			return reconcile.Result{}, err
		}

		// Check if this Deploy already exists
		foundDeploy := &appsv1.Deployment{}
		nsName := types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}
		err = r.client.Get(context.TODO(), nsName, foundDeploy)

		// If not exists, then create it
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", deploy.Namespace, "Deployment.Name", deploy.Name)
			err = r.client.Create(context.TODO(), deploy)
			if err != nil {
				// requeue with error
				return reconcile.Result{}, err
			}
			// Pod created successfully - don't requeue
			reqLogger.Info("Deploy created", "name", deploy.Name)
			time.Sleep(5 * time.Second)

			instance.Status.Phase = dronev1alpha1.PhaseDone
		} else if err != nil {
			// requeue with error
			return reconcile.Result{}, err
		} else {
			// Don't requeue because it will happen automatically when the pod status changes.
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
	// Don't requeue. We should be reconcile because either deploy or the CR changes.
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
			Name:      cr.Name,
			Namespace: cr.Namespace,
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
							Name:      cr.Spec.Template.Spec.Template.Spec.Containers[0].Name,
							Image:     cr.Spec.Template.Spec.Template.Spec.Containers[0].Image,
							Resources: cr.Spec.Template.Spec.Template.Spec.Containers[0].Resources,
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

func resultCallback(queueName string, body []byte){
	log.Info(" %s: Received a message: %s",queueName, string(body))

	result := &messaging.ResultMessage{}
	err := json.Unmarshal(body, result)

	log.Info(" SENDER: %s",result.Sender)
	if err != nil {
		log.Info(err.Error())
	}
	log.Info(" NAME: %s",result.LocalOffloading[0].Name)
}

func printCallback(queueName string, body []byte){
	log.Info(" %s: Received a message: %s",queueName, string(body))
}
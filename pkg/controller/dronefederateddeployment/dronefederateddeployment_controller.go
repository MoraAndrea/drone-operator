package dronefederateddeployment

import (
	"context"
	"drone-operator/drone-operator/pkg/controller/common/configuration"
	"drone-operator/drone-operator/pkg/controller/common/messaging"
	"encoding/json"
	errorstandard "errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	dronev1alpha1 "drone-operator/drone-operator/pkg/apis/drone/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_dronefederateddeployment")

var configurationEnv *configuration.ConfigType

var rabbit *messaging.RabbitMq

var advMessages []messaging.AdvertisementMessage

// Add creates a new DroneFederatedDeployment Controller and adds it to the Manager. The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	tmp := &ReconcileDroneFederatedDeployment{client: mgr.GetClient(), scheme: mgr.GetScheme()}
	tmp.init()
	return tmp
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
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

func (r *ReconcileDroneFederatedDeployment) init() {

	// Load and create configurationEnv
	configurationEnv = configuration.Config()
	rabbit = messaging.InitRabbitMq(configurationEnv)

	// Set consume queue
	rabbit.ConsumeMessage(configurationEnv.RabbitConf.QueueAdvertisementCtrl, r.advertisementCallback)
	rabbit.ConsumeMessage(configurationEnv.RabbitConf.QueueResult, r.resultCallback)

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
	log.Info("NamespacedName: " + request.Name)

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

		reqLogger.Info("It's time to Orchestrate!")

		// DRONE Agreement, send message Advertisement
		message := createAdvMessage(instance, messaging.ADD)
		rabbit.PublishMessage(message, configurationEnv.RabbitConf.QueueAdvertisement, false)

		//instance.Status.Phase = dronev1alpha1.PhaseRunning
	case dronev1alpha1.PhaseRunning:
		reqLogger.Info("Phase: RUNNING")

		/*err = r.deployContentCrd(instance, request.Name, request.Namespace)
		if err != nil {
			// requeue with error
			return reconcile.Result{}, err
		}*/

	case dronev1alpha1.PhaseDone:
		reqLogger.Info("Phase: DONE")
		//return reconcile.Result{}, nil
	default:
		reqLogger.Info("NOP")
		return reconcile.Result{}, nil
	}

	err = r.finalizeCheckInstance(instance)
	if err != nil {
		// requeue with error
		return reconcile.Result{}, err
	}

	// Update the DroneService instance, setting the status to the respective phase:
	//err = r.client.Status().Update(context.TODO(), instance)
	//if err != nil {
	//	return reconcile.Result{}, err
	//}
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

func createAdvMessage(cr *dronev1alpha1.DroneFederatedDeployment, typeMessage string) string {
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
	message := messaging.NewAdvertisementMessage(cr.Name, configurationEnv.Kubernetes.ClusterName, typeMessage, components)
	// log.Info(" Created Message %s", message)

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Error(err, "Error during marshal message...")
	}
	// log.Info(" Json Message: ", string(jsonData))
	adv := &messaging.AdvertisementMessage{}
	err1 := json.Unmarshal(jsonData, adv)
	println(adv.AppName)
	if err1 != nil {
		log.Error(err1, "Error during unmarshal message...")
	}
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

// newDeploy returns a deploy
func newDeployFromMessage(message *messaging.AdvertisementMessage) *appsv1.Deployment {

	log.Info("New deploy create.....")

	res:=corev1.ResourceRequirements{}
	res.Limits.Cpu().SetMilli(int64(message.Components[0].Function.Resources.Cpu*1000))
	res.Limits.Memory().Set(int64(message.Components[0].Function.Resources.Cpu*1000))
	res.Requests=nil

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      message.AppName,
			Namespace: configurationEnv.Kubernetes.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": message.Components[0].Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": message.Components[0].Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:      message.Components[0].Name,
							Image:     message.Components[0].Function.Image,
							Resources: res,
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

// Callbacks
//
// Callbacks for rabbitmq consume
func (r *ReconcileDroneFederatedDeployment) resultCallback(queueName string, body []byte) error {
	log.Info(" [x] Received a message: ", queueName, string(body))

	result := &messaging.ResultMessage{}
	err := json.Unmarshal(body, result)

	log.Info(" SENDER: " + result.Sender)
	if err != nil {
		log.Error(err, "Error during unmarshal result message...")
		return err
	}
	log.Info(" NAME LocalOffloading: " + result.LocalOffloading[0].AppName)

	if result.Sender==configurationEnv.Kubernetes.ClusterName{
		log.Info(" Cluster with cr, reconcile crd")
		//DEPLOY NO UPDATE
	} else {
		log.Info(" Cluster without cr, simple deploy.")
		err = r.updateStatusInstance(result.LocalOffloading[0].AppName, corev1.NamespaceDefault, dronev1alpha1.PhaseRunning)
		if err != nil {
			return err
		}
		//DEPLOY WITH UPDATE

		//deployContentAdv(findAdv(advMessages,result.LocalOffloading))
	}


	return nil
}

func (r *ReconcileDroneFederatedDeployment) advertisementCallback(queueName string, body []byte) error {
	log.Info(" [x] Received a message: ", queueName, string(body))

	adv := &messaging.AdvertisementMessage{}
	err := json.Unmarshal(body, adv)
	log.Info(" BaseNode Advertisement message: " + adv.BaseNode)
	if err != nil {
		log.Error(err, "Error during unmarshal adv message...")
		return err
	}
	if adv.Type == messaging.ADD {
		log.Info(" ADD: " + adv.AppName)
		advMessages, err = addAdv(advMessages, *adv)
		if err != nil {
			return err
		}
	}
	if adv.Type == messaging.DELETE {
		log.Info(" DELETE: " + adv.AppName)
		advMessages = removeAdv(advMessages, *adv)
	}
	return nil
}

func (r *ReconcileDroneFederatedDeployment) printCallback(queueName string, body []byte) {
	log.Info(" %s: Received a message: %s", queueName, string(body))
}

// K8S
//
// Function for manage K8S
func (r *ReconcileDroneFederatedDeployment) updateStatusInstance(name string, namespace string, phase string) error {
	log.Info(" UPDATE DRONE-FEDERATED: " + name + " --> phase: " + phase)
	namespacedName := types.NamespacedName{Name: name, Namespace: namespace}
	// Fetch the DroneFederatedDeployment instance
	instance := &dronev1alpha1.DroneFederatedDeployment{}
	err := r.client.Get(context.TODO(), namespacedName, instance)
	if err != nil {
		log.Error(err, " ERROR GET instance "+name)
		return err
	}
	instance.Status.Phase = phase
	// Update the DroneService instance, setting the status to the respective phase:
	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		log.Error(err, " ERROR UPDATE instance "+instance.Name)
		return err
	}
	return nil
}

func (r *ReconcileDroneFederatedDeployment) finalizeCheckInstance(cr *dronev1alpha1.DroneFederatedDeployment) error {
	log.Info(" Check finalize in: " + cr.Name)
	// name of our custom finalizer
	myFinalizerName := "finalizers.drone.com"

	// examine DeletionTimestamp to determine if object is under deletion
	if cr.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("DELETION-TIMESTAMP is ZERO")
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent registering our finalizer.
		if !containsString(cr.ObjectMeta.Finalizers, myFinalizerName) {
			log.Info("Set drone finalizer")
			cr.ObjectMeta.Finalizers = append(cr.ObjectMeta.Finalizers, myFinalizerName)
			if err := r.client.Update(context.TODO(), cr); err != nil {
				return err
			}
		}
	} else {
		log.Info("DELETION-TIMESTAMP not ZERO")
		// The object is being deleted
		if containsString(cr.ObjectMeta.Finalizers, myFinalizerName) {
			log.Info("CONTAINS OUR FINALIZER")
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteExternalResources(cr); err != nil {
				// if fail to delete the external dependency here, return with error so that it can be retried
				return err
			}

			// remove our finalizer from the list and update it.
			cr.ObjectMeta.Finalizers = removeString(cr.ObjectMeta.Finalizers, myFinalizerName)
			if err := r.client.Update(context.TODO(), cr); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ReconcileDroneFederatedDeployment) deployContentAdv( adv messaging.AdvertisementMessage, namespace string) error {
	log.Info(" DEPLOY: "+adv.AppName+" in "+namespace)
	deploy:=newDeployFromMessage(&adv)

	jsonData, err := json.Marshal(deploy)
	if err != nil {
		log.Error(err, "Error during marshal message...")
	}
	log.Info("[D] deploy: " + string(jsonData))

	// Check if this Deploy already exists
	foundDeploy := &appsv1.Deployment{}
	nsName := types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}
	err = r.client.Get(context.TODO(), nsName, foundDeploy)

	// If not exists, then create it
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new Deployment", "Deployment.Namespace", deploy.Namespace, "Deployment.Name", deploy.Name)
		err = r.client.Create(context.TODO(), deploy)
		if err != nil {
			// requeue with error
			return err
		}
		// Pod created successfully - don't requeue
		log.Info("Deploy created", "name", deploy.Name)
		time.Sleep(5 * time.Second)

		//instance.Status.Phase = dronev1alpha1.PhaseDone
		err = r.updateStatusInstance(adv.AppName, namespace, dronev1alpha1.PhaseDone)
		if err != nil {
			// requeue with error
			return err
		}

	} else if err != nil {
		// requeue with error
		return err
	} else {
		// Don't requeue because it will happen automatically when the pod status changes.
		return nil
	}

	return nil
}

func (r *ReconcileDroneFederatedDeployment) deployContentAdvNoCR( adv messaging.AdvertisementMessage, namespace string) error {
	log.Info(" DEPLOY: "+adv.AppName+" in "+namespace)
	deploy:=newDeployFromMessage(&adv)

	jsonData, err := json.Marshal(deploy)
	if err != nil {
		log.Error(err, "Error during marshal message...")
	}
	log.Info("[D] deploy: " + string(jsonData))

	// Check if this Deploy already exists
	foundDeploy := &appsv1.Deployment{}
	nsName := types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}
	err = r.client.Get(context.TODO(), nsName, foundDeploy)

	// If not exists, then create it
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new Deployment", "Deployment.Namespace", deploy.Namespace, "Deployment.Name", deploy.Name)
		err = r.client.Create(context.TODO(), deploy)
		if err != nil {
			return err
		}
		log.Info("Deploy created", "name", deploy.Name)
		time.Sleep(5 * time.Second)

	} else if err != nil {
		return err
	} else {
		return nil
	}

	return nil
}


func (r *ReconcileDroneFederatedDeployment) deployContentCrd(cr *dronev1alpha1.DroneFederatedDeployment, name string, namespace string) error {
	log.Info(" DEPLOY: "+cr.Name+" in "+cr.Namespace)

	//pod := newPodForCR(instance)
	deploy := newDeployForCR(cr)
	jsonData, err1 := json.Marshal(deploy)
	if err1 != nil {
		log.Error(err1, "Error during marshal message...")
	}
	log.Info("[D] deploy: " + string(jsonData))

	// Set DroneService instance as the owner and controller
	err := controllerutil.SetControllerReference(cr, deploy, r.scheme)
	if err != nil {
		// requeue with error
		return err
	}

	// Check if this Deploy already exists
	foundDeploy := &appsv1.Deployment{}
	nsName := types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}
	err = r.client.Get(context.TODO(), nsName, foundDeploy)

	// If not exists, then create it
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new Deployment", "Deployment.Namespace", deploy.Namespace, "Deployment.Name", deploy.Name)
		err = r.client.Create(context.TODO(), deploy)
		if err != nil {
			// requeue with error
			return err
		}
		// Pod created successfully - don't requeue
		log.Info("Deploy created", "name", deploy.Name)
		time.Sleep(5 * time.Second)

		//instance.Status.Phase = dronev1alpha1.PhaseDone
		err = r.updateStatusInstance(name, namespace, dronev1alpha1.PhaseDone)
		if err != nil {
			// requeue with error
			return err
		}

	} else if err != nil {
		// requeue with error
		return err
	} else {
		// Don't requeue because it will happen automatically when the pod status changes.
		return nil
	}

	return nil
}

func (r *ReconcileDroneFederatedDeployment) deleteExternalResources(cr *dronev1alpha1.DroneFederatedDeployment) error {
	log.Info(" DELETE RESOURCE: " + cr.Name)
	// delete any external resources associated with the DroneFederatedDeployment
	// Ensure that delete implementation is idempotent and safe to invoke multiple types for same object.

	// Send message DELETING
	message := createAdvMessage(cr, messaging.DELETE)
	rabbit.PublishMessage(message, configurationEnv.RabbitConf.QueueAdvertisement, false)
	return nil
}

// Helpers
//
// functions to check and remove string from a slice of strings.

// Check string from slide of strings
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
// Remove string from slide of strings
func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

// functions to add or remove message from adv slice.
func addAdv(slice []messaging.AdvertisementMessage, s messaging.AdvertisementMessage) (result []messaging.AdvertisementMessage,e error) {
	for _, item := range slice {
		if s.Equal(item) {
			return advMessages, errorstandard.New("Message already present for app: " + s.AppName)
		}
	}
	advMessages = append(advMessages, s)
	return advMessages, nil
}

func findAdv(slice []messaging.AdvertisementMessage, appName string, nameComponent string) (result messaging.AdvertisementMessage) {
	for _, item := range slice {
		if item.AppName==appName && item.Components[0].Name==nameComponent {
			return item
		}
	}
	return
}

func removeAdv(slice []messaging.AdvertisementMessage, s messaging.AdvertisementMessage) (result []messaging.AdvertisementMessage) {
	for _, item := range slice {
		if s.Equal(item) {
			continue
		}
		result = append(result, item)
	}
	return
}

//Utils

func timeUntilSchedule(schedule string) (time.Duration, error) {
	now := time.Now().UTC()
	layout := "2006-01-02T15:04:05Z"
	s, err := time.Parse(layout, schedule)
	if err != nil {
		return time.Duration(0), err
	}
	return s.Sub(now), nil
}
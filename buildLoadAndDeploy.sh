echo "Build Application, Create docker..."
operator-sdk build drone-operator:first

echo "Load image on kind..."
kind load docker-image drone-operator:first --name cluster1

echo "Apply operator and other utils..."
# Setup Service Account
kubectl apply -f deploy/service_account.yaml -n drone
# Setup RBAC
kubectl apply -f deploy/role.yaml -n drone
kubectl apply -f deploy/role_binding.yaml -n drone
# Setup the CRD
kubectl apply -f deploy/crds/drone_v1alpha1_dronefederateddeployment_crd.yaml -n drone
# Deploy the app-operator
kubectl apply -f deploy/operator.yaml -n drone


# Create an DroneFederatedDeployment CR
# echo "Create ad DroneFederatedDeployment"
# kubectl apply -f deploy/crds/app.example.com_v1alpha1_appservice_cr.yaml -n drone
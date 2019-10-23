echo "Build Application, Create docker..."
go operator-sdk build drone-operator:first

echo "Load image on kind..."
kind load docker-image drone-operator:first --name cluster1

echo "Apply operator and other utils..."
# Setup Service Account
kubectl create -f deploy/service_account.yaml
# Setup RBAC
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
# Setup the CRD
kubectl create -f deploy/crds/app.example.com_appservices_crd.yaml
# Deploy the app-operator
kubectl create -f deploy/operator.yaml


# Create an DroneFederatedDeployment CR
# echo "Create ad DroneFederatedDeployment"
# kubectl create -f deploy/crds/app.example.com_v1alpha1_appservice_cr.yaml
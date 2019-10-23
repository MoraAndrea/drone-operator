package controller

import (
	"drone-operator/drone-operator/pkg/controller/common/messaging"
	"drone-operator/drone-operator/pkg/controller/dronefederateddeployment"
	"github.com/prometheus/common/log"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, dronefederateddeployment.Add)

	// Init rabbitMq connection, channel and queue
	var rabbit = messaging.InitRabbitMq()
	log.Info( "AAAAAAAAAAAAAAAAAAAAAAAAAAAA ",rabbit.Q.Name)

	rabbit.PublishMessage("ciao","app-advertisement", false)


}

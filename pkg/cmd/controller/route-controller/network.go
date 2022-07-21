package route

import (
	"fmt"
	"net"
	"time"

	"github.com/openshift/openshift-controller-manager/pkg/route/ingressip"
)

func getIngressIPController(ctx *ControllerContext) (*ingressip.IngressIPController, error) {
	// TODO configurable?
	resyncPeriod := 10 * time.Minute

	if len(ctx.OpenshiftControllerConfig.Ingress.IngressIPNetworkCIDR) == 0 {
		return nil, nil
	}

	_, ipNet, err := net.ParseCIDR(ctx.OpenshiftControllerConfig.Ingress.IngressIPNetworkCIDR)
	if err != nil {
		return nil, fmt.Errorf("unable to start ingress IP controller: %v", err)
	}

	if ipNet.IP.IsUnspecified() {
		// TODO: Is this an error?
		return nil, nil
	}

	ingressIPController := ingressip.NewIngressIPController(
		ctx.KubernetesInformers.Core().V1().Services().Informer(),
		ctx.ClientBuilder.ClientOrDie(infraServiceIngressIPControllerServiceAccountName),
		ipNet,
		resyncPeriod,
	)
	//go ingressIPController.Run(ctx.Stop)

	return ingressIPController, nil
}

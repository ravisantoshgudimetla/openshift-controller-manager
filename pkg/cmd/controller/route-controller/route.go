package route

import (
	"context"
	"os"

	"github.com/openshift/openshift-controller-manager/pkg/route/ingress"
	"github.com/openshift/openshift-controller-manager/pkg/route/ingressip"
	v1 "k8s.io/api/core/v1"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

func RunRouteControllerManager(ctx *ControllerContext) (bool, error) {
	kubeClient, err := ctx.ClientBuilder.Client(infraIngressToRouteControllerServiceAccountName)
	if err != nil {
		return true, err
	}
	config := ctx.OpenshiftControllerConfig
	var ingressToRouteController *ingress.Controller
	var ingressController *ingressip.IngressIPController
	if ctx.IsControllerEnabled("openshift.io/ingress-ip") {
		ingressController, err = getIngressIPController(ctx)
		if err != nil {
			return true, err
		}
	}
	if ctx.IsControllerEnabled("openshift.io/ingress-to-route") {
		ingressToRouteController, err = getIngressToRouteController(ctx)
		if err != nil {
			return true, err
		}
	}
	routeControllerManager := func(cntx context.Context) {
		if ingressController != nil {
			go ingressController.Run(cntx.Done())
		}
		if ingressToRouteController != nil {
			go ingressToRouteController.Run(5, cntx.Done())
		}
	}
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "cluster-route-controller"})
	id, err := os.Hostname()
	if err != nil {
		return false, err
	}
	// Create a new lease for the route controller manager
	rl, err := resourcelock.New(
		"configmapsleases",
		"openshift-route-controller-manager", // TODO: This namespace needs to be created by ocm for now.
		"openshift-route-controllers",
		kubeClient.CoreV1(),
		kubeClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: eventRecorder,
		})
	if err != nil {
		return false, err
	}
	go leaderelection.RunOrDie(context.Background(),
		leaderelection.LeaderElectionConfig{
			Lock:            rl,
			ReleaseOnCancel: true,
			LeaseDuration:   config.LeaderElection.LeaseDuration.Duration,
			RenewDeadline:   config.LeaderElection.RenewDeadline.Duration,
			RetryPeriod:     config.LeaderElection.RetryPeriod.Duration,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: routeControllerManager,
				OnStoppedLeading: func() {
					klog.Fatalf("leaderelection lost")
				},
			},
		})
	return true, nil
}

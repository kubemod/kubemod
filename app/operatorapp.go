/*
Licensed under the BSD 3-Clause License (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://opensource.org/licenses/BSD-3-Clause

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"github.com/go-logr/logr"
	apiv1beta1 "github.com/kubemod/kubemod/api/v1beta1"
	"github.com/kubemod/kubemod/controllers"
	"github.com/kubemod/kubemod/core"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// KubeModOperatorApp is the DI container of kubemod operator application state.
type KubeModOperatorApp struct {
}

// NewKubeModOperatorApp instantiates a kubemod application.
func NewKubeModOperatorApp(
	scheme *runtime.Scheme,
	manager manager.Manager,
	modRuleReconciler *controllers.ModRuleReconciler,
	coreDragnetWebhookHandler *core.DragnetWebhookHandler,
	corePodBindingWebhookHandler *core.PodBindingWebhookHandler,
	log logr.Logger,
) (*KubeModOperatorApp, error) {

	setupLog := log.WithName("operator-setup")

	// Set up the ModRuleReconciler with the manager.
	err := modRuleReconciler.SetupWithManager(manager)

	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ModRule")
		return nil, err
	}

	// Wire up the ModRule web hooks.
	if err := (&apiv1beta1.ModRule{}).SetupWebhookWithManager(manager); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ModRule")
		return nil, err
	}

	// Wire up the core web hook.
	hookServer := manager.GetWebhookServer()
	setupLog.Info("registering core mutating webhook")

	hookServer.Register(
		"/dragnet-webhook",
		&webhook.Admission{
			Handler: coreDragnetWebhookHandler,
		},
	)

	hookServer.Register(
		"/podbinding-webhook",
		&webhook.Admission{
			Handler: corePodBindingWebhookHandler,
		},
	)

	// Start the manager.
	setupLog.Info("starting manager")
	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return nil, err
	}

	return &KubeModOperatorApp{},
		nil
}

// NewControllerManager instantiates a new controller manager.
func NewControllerManager(scheme *runtime.Scheme, metricsAddr OperatorMetricsAddr, healthProbeAddr OperatorHealthProbeAddr, enableLeaderElection EnableLeaderElection, log logr.Logger) (manager.Manager, error) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     string(metricsAddr),
		HealthProbeBindAddress: string(healthProbeAddr),
		Port:                   9443,
		LeaderElection:         bool(enableLeaderElection),
		LeaderElectionID:       "f950e141.kubemod.io",
	})
	setupLog := log.WithName("operator-setup")

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return nil, err
	}

	setupLog.Info("health server is starting to listen", "addr", healthProbeAddr)

	// Add /healthz health-check endpoint
	if err := mgr.AddHealthzCheck("default", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		return nil, err
	}

	// Add /readyz health-check endpoint
	if err := mgr.AddReadyzCheck("tracker", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create ready check")
		return nil, err
	}

	return mgr, nil
}

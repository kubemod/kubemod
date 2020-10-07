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
	"time"

	"github.com/go-logr/logr"
	apiv1beta1 "github.com/kubemod/kubemod/api/v1beta1"
	"github.com/kubemod/kubemod/controllers"
	"github.com/kubemod/kubemod/core"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	kzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// KubeModApp is the DI container of kubemod application state.
type KubeModApp struct {
}

// NewKubeModApp instantiates a kubemod application.
func NewKubeModApp(
	scheme *runtime.Scheme,
	manager manager.Manager,
	modRuleReconciler *controllers.ModRuleReconciler,
	coreDragnetWebhookHandler *core.DragnetWebhookHandler,
	log logr.Logger,
) (*KubeModApp, error) {

	setupLog := log.WithName("setup")

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

	// Start the manager.
	setupLog.Info("starting manager")
	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return nil, err
	}

	return &KubeModApp{},
		nil
}

// NewControllerManager instantiates a new controller manager.
func NewControllerManager(scheme *runtime.Scheme, metricsAddr string, enableLeaderElection EnableLeaderElection, log logr.Logger) (manager.Manager, error) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     bool(enableLeaderElection),
		LeaderElectionID:   "f950e141.kubemod.io",
	})
	setupLog := log.WithName("setup")

	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return nil, err
	}

	return mgr, nil
}

func iso8601TimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000Z0700"))
}

// NewLogger instantiates a new logger.
func NewLogger(enableDevModeLog EnableDevModeLog) logr.Logger {
	log := kzap.New(func(o *kzap.Options) {
		o.Development = bool(enableDevModeLog)

		// Enforce JSON encoder for both debug and non-debug deployments,
		// but ensure that timing is encoded using ISO8601.
		encCfg := zap.NewProductionEncoderConfig()
		encCfg.EncodeTime = iso8601TimeEncoder
		o.Encoder = zapcore.NewJSONEncoder(encCfg)
	})

	// Wire up the controller with the log.
	ctrl.SetLogger(log)

	return log
}

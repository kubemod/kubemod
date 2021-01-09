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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apiv1beta1 "github.com/kubemod/kubemod/api/v1beta1"
	"github.com/kubemod/kubemod/core"
)

// ClusterModRulesNamespace is a type of string used by DI to inject the namespace where cluster-wide ModRules are deployed.
type ClusterModRulesNamespace string

// ModRuleReconciler reconciles a ModRule object
type ModRuleReconciler struct {
	client                   client.Client
	log                      logr.Logger
	scheme                   *runtime.Scheme
	modRuleStore             *core.ModRuleStore
	clusterModRulesNamespace string
}

// NewModRuleReconciler creates a new ModRuleReconciler.
func NewModRuleReconciler(manager manager.Manager, modRuleStore *core.ModRuleStore, clusterModRulesNamespace ClusterModRulesNamespace, log logr.Logger) (*ModRuleReconciler, error) {

	reconciler := &ModRuleReconciler{
		client:                   manager.GetClient(),
		log:                      log.WithName("controllers").WithName("modrule"),
		scheme:                   manager.GetScheme(),
		modRuleStore:             modRuleStore,
		clusterModRulesNamespace: string(clusterModRulesNamespace),
	}

	return reconciler, nil
}

// +kubebuilder:rbac:groups=api.kubemod.io,resources=modrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=api.kubemod.io,resources=modrules/status,verbs=get;update;patch

// Reconcile performs ModRule reconciliation.
func (r *ModRuleReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var modRule apiv1beta1.ModRule
	ctx := context.Background()
	log := r.log.WithValues("modrule", req.NamespacedName)

	storeNamespace := req.Namespace

	// If the ModRule is stored in the cluster-wide namespace, store the ModRule under an empty namespace
	// in order to match non-namespaced resources.
	if storeNamespace == r.clusterModRulesNamespace {
		storeNamespace = ""
	}

	if err := r.client.Get(ctx, req.NamespacedName, &modRule); err != nil {

		// If the modrule is not found, then it has been deleted.
		if apierrors.IsNotFound(err) {
			// Delete the ModRule from the ModRule memory store.
			r.modRuleStore.Delete(storeNamespace, req.Name)
		} else {
			log.Error(err, "unable to fetch ModRule")
		}

		// Ignore not-found errors since they can't be fixed by an immediate requeue
		// (we will need to wait for a new notification), and we can get them on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound((err))
	}

	// Store the new ModRule in our memory store, but before we do, clear the namespace
	// in case the ModRule is deployed to the cluster-wide namespace.
	modRule.Namespace = storeNamespace
	r.modRuleStore.Put(&modRule)

	log.V(1).Info("Successfully stored ModRule")

	return ctrl.Result{}, nil
}

// SetupWithManager hooks up our controller with the controller manager.
func (r *ModRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1beta1.ModRule{}).
		Complete(r)
}

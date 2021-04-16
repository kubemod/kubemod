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

package v1beta1

import (
	"fmt"
	"regexp"

	"github.com/kubemod/kubemod/expressions"
	"github.com/kubemod/kubemod/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var (
	modrulelog       = logf.Log.WithName("modrule-resource")
	jsonPathLanguage = expressions.NewJSONPathLanguage()
)

// SetupWebhookWithManager hooks up the web hook with a manager.
func (r *ModRule) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

var _ webhook.Defaulter = &ModRule{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ModRule) Default() {
	modrulelog.V(1).Info("default", "name", r.Name)

	for i := range r.Spec.Match {
		mi := &r.Spec.Match[i]
		if mi.MatchFor == "" {
			mi.MatchFor = MatchForTypeAny
		}
	}
}

var _ webhook.Validator = &ModRule{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ModRule) ValidateCreate() error {
	modrulelog.V(1).Info("validate create", "name", r.Name)

	return r.validateModRule()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ModRule) ValidateUpdate(old runtime.Object) error {
	modrulelog.V(1).Info("validate update", "name", r.Name)

	return r.validateModRule()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ModRule) ValidateDelete() error {
	modrulelog.V(1).Info("validate delete", "name", r.Name)

	return nil
}

func (r *ModRule) validateModRule() error {
	var (
		allErrs field.ErrorList
		err     error
	)

	if r.Spec.Type != ModRuleTypePatch && r.Spec.Type != ModRuleTypeReject {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("type"), r.Spec.Type, "unrecognized ModRule type"))
	}

	if r.Spec.Type != ModRuleTypePatch && len(r.Spec.Patch) > 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("patch"), r.Spec.Patch, "field 'patch' should be present only for ModRules of type Patch"))
	}

	if r.Spec.Type == ModRuleTypePatch && len(r.Spec.Patch) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("patch"), "field 'patch' cannot be empty for ModRules of type Patch"))
	}

	if r.Spec.Type != ModRuleTypeReject && r.Spec.RejectMessage != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("rejectMessage"), *r.Spec.RejectMessage, "field 'rejectMessage' should be present only for ModRules of type Reject"))
	}

	// Validate the ModRule match items.
	for i, matchItem := range r.Spec.Match {
		// match.select is required.
		if matchItem.Select == "" {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("match").Index(i).Child("select"), matchItem.Select, "spec.match[].select in body must be non-empty string"))
		} else {
			// Test the match query.
			_, err = jsonPathLanguage.NewEvaluable(matchItem.Select)

			if err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("match").Index(i).Child("select"), matchItem.Select, fmt.Sprintf("%v", err)))
			}
		}

		// Then the optional target regexp.
		if matchItem.MatchRegex != nil {
			_, err = regexp.Compile(*matchItem.MatchRegex)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("match").Index(i).Child("matchRegex"), *matchItem.MatchRegex, fmt.Sprintf("%v", err)))
			}
		}

		if matchItem.MatchFor != MatchForTypeAny && matchItem.MatchFor != MatchForTypeAll {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("match").Index(i).Child("matchFor"), matchItem.MatchFor, "unrecognized matchFor value"))
		}
	}

	// Validate the patch value templates and optional select queries.
	for i, po := range r.Spec.Patch {
		if po.Value != nil {
			value := *po.Value

			// Test the template.
			_, err := util.NewSafeTemplate(po.Path).Parse(util.PreProcessModRuleGoTemplate(value))

			if err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("patch").Index(i).Child("value"), value, fmt.Sprintf("%v", err)))
			}

			// Test the select query.
			if po.Select != nil && *po.Select != "" {
				_, err = jsonPathLanguage.NewEvaluable(*po.Select)

				if err != nil {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("patch").Index(i).Child("select"), *po.Select, fmt.Sprintf("%v", err)))
				}
			}
		}
	}

	// Validate the rejectMessage as a template.
	if r.Spec.RejectMessage != nil {
		_, err = util.NewSafeTemplate("rejectMessage").Parse(*r.Spec.RejectMessage)

		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("rejectMessage"), *r.Spec.RejectMessage, fmt.Sprintf("%v", err)))
		}
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: "api.kubemod.io", Kind: "ModRule"},
			r.Name,
			allErrs)
	}

	return nil
}

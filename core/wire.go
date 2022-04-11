//go:build wireinject
// +build wireinject

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

package core

import (
	"github.com/kubemod/kubemod/expressions"
	"github.com/kubemod/kubemod/util"

	"github.com/go-logr/logr"
	"github.com/google/wire"
)

// ModRuleStoreTestBed is a structure used to provide services for ModRuleStore tests.
type ModRuleStoreTestBed struct {
	modRuleStore *ModRuleStore
}

// ModRuleStoreItemTestBed is used in ModRuleStoreItem test.
type ModRuleStoreItemTestBed struct {
	itemFactory *ModRuleStoreItemFactory
}

// NewLogger instantiates a new testing logger.
func NewTestLogger(tLogger util.TLogger) logr.Logger {
	return util.TestLogger{TLogger: tLogger}
}

// NewModRuleStoreTestBed instantiates a new test bed.
func NewModRuleStoreTestBed(modRuleStore *ModRuleStore) *ModRuleStoreTestBed {
	return &ModRuleStoreTestBed{
		modRuleStore: modRuleStore,
	}
}

// NewModRuleStoreItemTestBed instantiates a new test bed.
func NewModRuleStoreItemTestBed(itemFactory *ModRuleStoreItemFactory) *ModRuleStoreItemTestBed {
	return &ModRuleStoreItemTestBed{
		itemFactory: itemFactory,
	}
}

// InitializeModRuleStoreTestBed instructs wire how to construct a new test bed.
func InitializeModRuleStoreTestBed(clusterModRulesNamespace ClusterModRulesNamespace, tLogger util.TLogger) *ModRuleStoreTestBed {
	wire.Build(
		NewModRuleStoreTestBed,
		NewTestLogger,
		expressions.NewKubeModJSONPathLanguage,
		NewModRuleStoreItemFactory,
		NewModRuleStore,
	)

	return nil
}

// InitializeModRuleStoreItemTestBed instructs wire how to construct a new test bed.
func InitializeModRuleStoreItemTestBed(tLogger util.TLogger) *ModRuleStoreItemTestBed {
	wire.Build(
		NewModRuleStoreItemTestBed,
		NewTestLogger,
		expressions.NewKubeModJSONPathLanguage,
		NewModRuleStoreItemFactory,
	)

	return nil
}

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
	"github.com/golang/mock/gomock"
	"github.com/kubemod/kubemod/expressions"
	"github.com/kubemod/kubemod/mocks"
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

// DragnetWebhookHandlerTestBed is used in DragnetWebhookHandler test.
type DragnetWebhookHandlerTestBed struct {
	mockCtrl      *gomock.Controller
	mockK8sClient *mocks.MockClient
	modRuleStore  *ModRuleStore
	log           logr.Logger
}

// NewLogger instantiates a new testing logger.
func NewTestLogger(tLogger util.TLogger) logr.Logger {
	return util.TestLogger{TLogger: tLogger}
}

// NewMockTestReporter converts a TLogger to gomock.TestReporter.
func NewMockTestReporter(tLogger util.TLogger) gomock.TestReporter {
	return tLogger
}

// NewGoMockController instantiates a new mock controller.
func NewGoMockController(testReporter gomock.TestReporter) *gomock.Controller {
	return gomock.NewController(testReporter)
}

// NewK8sMockClient instantiates a new K8S client mock.
func NewK8sMockClient(mockCtrl *gomock.Controller) *mocks.MockClient {
	return mocks.NewMockClient(mockCtrl)
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

// NewDragnetWebhookHandlerTestBed instantiates a new test bed.
func NewDragnetWebhookHandlerTestBed(modRuleStore *ModRuleStore, mockCtrl *gomock.Controller, mockK8sClient *mocks.MockClient, log logr.Logger) *DragnetWebhookHandlerTestBed {
	return &DragnetWebhookHandlerTestBed{
		mockCtrl:      mockCtrl,
		mockK8sClient: mockK8sClient,
		modRuleStore:  modRuleStore,
		log:           log,
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

// InitializeDragnetWebhookHandlerTestBed instructs wire how to construct a new test bed.
func InitializeDragnetWebhookHandlerTestBed(clusterModRulesNamespace ClusterModRulesNamespace, tLogger util.TLogger) *DragnetWebhookHandlerTestBed {
	wire.Build(
		NewDragnetWebhookHandlerTestBed,
		NewMockTestReporter,
		NewGoMockController,
		NewK8sMockClient,
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

// Copyright 2021 ADA Logics Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package sensor

import (
	"context"
	"sync"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-events/common/logging"
	eventbusv1alpha1 "github.com/argoproj/argo-events/pkg/apis/eventbus/v1alpha1"
	"github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"
)

var initter sync.Once

func initFunc() {
	_ = eventbusv1alpha1.AddToScheme(scheme.Scheme)
	_ = v1alpha1.AddToScheme(scheme.Scheme)
	_ = appv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
}

func FuzzSensorController(data []byte) int {
	initter.Do(initFunc)
	testImage := "test-image"
	f := fuzz.NewConsumer(data)
	sensorObj := &v1alpha1.Sensor{}
	err := f.GenerateStruct(sensorObj)
	if err != nil {
		return 0
	}
	ctx := context.Background()
	cl := fake.NewClientBuilder().Build()
	r := &reconciler{
		client:      cl,
		scheme:      scheme.Scheme,
		sensorImage: testImage,
		logger:      logging.NewArgoEventsLogger(),
	}
	_ = r.reconcile(ctx, sensorObj)
	return 1
}

func FuzzSensorControllerReconcile(data []byte) int {
	initter.Do(initFunc)
	testImage := "test-image"
	f := fuzz.NewConsumer(data)
	testBus := &eventbusv1alpha1.EventBus{}
	err := f.GenerateStruct(testBus)
	if err != nil {
		return 0
	}
	sensorObj := &v1alpha1.Sensor{}
	err = f.GenerateStruct(sensorObj)
	if err != nil {
		return 0
	}
	testLabels := make(map[string]string)
	err = f.FuzzMap(&testLabels)
	if err != nil {
		return 0
	}
	ctx := context.Background()
	cl := fake.NewClientBuilder().Build()
	err = cl.Create(ctx, testBus)
	if err != nil {
		return 0
	}
	args := &AdaptorArgs{
		Image:  testImage,
		Sensor: sensorObj,
		Labels: testLabels,
	}
	_ = Reconcile(cl, args, logging.NewArgoEventsLogger())
	return 1
}

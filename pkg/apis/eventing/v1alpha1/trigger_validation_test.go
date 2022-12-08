/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "knative.dev/eventing-rabbitmq/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing-rabbitmq/pkg/utils"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/client/clientset/versioned/fake"
	"knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/pkg/apis"
)

const parallelismAnnotation = "rabbitmq.eventing.knative.dev/parallelism"

func TestTriggerValidate(t *testing.T) {
	tests := []struct {
		name     string
		trigger  *v1.RabbitTrigger
		original *v1.RabbitTrigger
		err      *apis.FieldError
		objects  []runtime.Object
	}{
		{
			name:    "broker not found gets ignored",
			trigger: trigger(withBroker("foo")),
		},
		{
			name: "different broker class gets ignored",
			trigger: trigger(
				withBroker("foo"),
			),
			objects: []runtime.Object{
				&eventingv1.Broker{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "foo",
						Annotations: map[string]string{eventingv1.BrokerClassAnnotationKey: "some-other-broker"},
					},
				},
			},
		},
		{
			name:     "filters are immutable",
			trigger:  trigger(withBroker("foo")),
			original: trigger(withBroker("foo"), withFilters(filter("x", "y"))),
			err: &apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{"spec", "filter"},
				Details: "{*v1.TriggerFilter}:\n\t-: \"&{Attributes:map[x:y]}\"\n\t+: \"<nil>\"\n",
			},
			objects: []runtime.Object{
				validBroker("foo"),
			},
		},
		{
			name:    "out of bounds parallelism count annotation",
			trigger: trigger(withBroker("foo"), withParallelism("0")),
			err:     apis.ErrOutOfBoundsValue(0, 1, 1000, parallelismAnnotation),
			objects: []runtime.Object{
				validBroker("foo"),
			},
		},
		{
			name:    "invalid parallelism count annotation",
			trigger: trigger(withBroker("foo"), withParallelism("notAnumber")),
			err: &apis.FieldError{
				Message: "Failed to parse valid int from parallelismAnnotation",
				Paths:   []string{"metadata", "annotations", parallelismAnnotation},
				Details: `strconv.Atoi: parsing "notAnumber": invalid syntax`,
			},
			objects: []runtime.Object{
				validBroker("foo"),
			},
		},
		{
			name:     "update parallelism count annotation",
			trigger:  trigger(withBroker("foo"), withParallelism("100")),
			original: trigger(withBroker("foo")),
			objects: []runtime.Object{
				validBroker("foo"),
			},
		},
		{
			name:    "valid Parallelisp count annotation",
			trigger: trigger(withBroker("foo"), withParallelism("100")),
			objects: []runtime.Object{
				validBroker("foo"),
			},
		},
		{
			name:    "invalid resource annotations",
			trigger: trigger(withBroker("foo"), withAnnotation(utils.CPURequestAnnotation, "invalid")),
			err: &apis.FieldError{
				Message: "Failed to parse quantity from rabbitmq.eventing.knative.dev/cpu-request",
				Paths:   []string{"metadata", "annotations", "rabbitmq.eventing.knative.dev/cpu-request"},
				Details: "quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'",
			},
			objects: []runtime.Object{
				validBroker("foo"),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), client.Key{}, fake.NewSimpleClientset(tc.objects...))
			if tc.original != nil {
				t := eventingv1.Trigger{
					TypeMeta:   tc.original.TypeMeta,
					ObjectMeta: tc.original.ObjectMeta,
					Spec:       tc.original.Spec,
					Status:     tc.original.Status,
				}
				ctx = apis.WithinUpdate(ctx, &t)
			}

			err := tc.trigger.Validate(ctx)
			if diff := cmp.Diff(tc.err, err, cmpopts.IgnoreUnexported(apis.FieldError{})); diff != "" {
				t.Error("Trigger.Validate (-want, +got) =", diff)
			}
		})
	}
}

type triggerOpt func(*v1.RabbitTrigger)

func trigger(opts ...triggerOpt) *v1.RabbitTrigger {
	t := &v1.RabbitTrigger{
		Spec: eventingv1.TriggerSpec{},
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

func filter(k, v string) []string {
	return []string{k, v}
}

func withFilters(filters ...[]string) triggerOpt {
	return func(t *v1.RabbitTrigger) {
		if t.Spec.Filter == nil {
			t.Spec.Filter = &eventingv1.TriggerFilter{}
		}
		if t.Spec.Filter.Attributes == nil {
			t.Spec.Filter.Attributes = map[string]string{}
		}
		for _, filter := range filters {
			t.Spec.Filter.Attributes[filter[0]] = filter[1]
		}
	}
}

func withParallelism(parallelism string) triggerOpt {
	return func(t *v1.RabbitTrigger) {
		if t.Annotations == nil {
			t.Annotations = map[string]string{}
		}

		t.Annotations[parallelismAnnotation] = parallelism
	}
}

func withAnnotation(key, value string) triggerOpt {
	return func(t *v1.RabbitTrigger) {
		if t.Annotations == nil {
			t.Annotations = map[string]string{}
		}

		t.Annotations[key] = value
	}
}

func withBroker(name string) triggerOpt {
	return func(t *v1.RabbitTrigger) {
		t.Spec.Broker = name
	}
}

func validBroker(name string) *eventingv1.Broker {
	return &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{eventingv1.BrokerClassAnnotationKey: v1.BrokerClass},
		},
	}
}

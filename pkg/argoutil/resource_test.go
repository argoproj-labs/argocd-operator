// Copyright 2019 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argoutil

import (
	"reflect"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

func TestDefaultAnnotations(t *testing.T) {
	type args struct {
		cr *argoproj.ArgoCD
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "simple annotations",
			args: args{
				&argoproj.ArgoCD{
					ObjectMeta: v1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
					},
				},
			},
			want: map[string]string{
				"argocds.argoproj.io/name":      "foo",
				"argocds.argoproj.io/namespace": "bar",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := common.DefaultAnnotations(tt.args.cr.Name, tt.args.cr.Namespace); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DefaultAnnotations() = %v, want %v", got, tt.want)
			}
		})
	}
}

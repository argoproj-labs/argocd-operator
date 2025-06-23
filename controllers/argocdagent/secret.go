// Copyright 2025 ArgoCD Operator Developers
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

package argocdagent

import (
	"context"
	"fmt"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReconcileRedisSecret(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {
	secret := argoutil.NewSecretWithSuffix(cr, "redis")
	exists := true
	if err := argoutil.FetchObject(client, cr.Namespace, secret.Name, secret); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing redis secret %s in namespace %s: %v", secret.Name, cr.Namespace, err)
		}
		exists = false
	}

	if !exists {
		secret.Data = map[string][]byte{
			// TODO: Not sure if hardcoding this is the best way to do this
			"auth": []byte("kpQyNY-jche7EBuW"),
		}

		if err := client.Create(context.TODO(), secret); err != nil {
			return fmt.Errorf("failed to create redis secret %s in namespace %s: %v", secret.Name, cr.Namespace, err)
		}
		return nil
	}

	return nil
}

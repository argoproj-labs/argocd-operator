// Copyright 2021 ArgoCD Operator Developers
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

package argocd

import (
	"sync"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
)

var (
	mutex sync.RWMutex
	hooks = []Hook{}
)

// Hook changes resources as they are created or updated by the
// reconciler.
type Hook func(*argoprojv1alpha1.ArgoCD, interface{}, string) error

// Register adds a modifier for updating resources during reconciliation.
func Register(h ...Hook) {
	mutex.Lock()
	defer mutex.Unlock()
	hooks = append(hooks, h...)
}

// ApplyReconcilerHook applies custom reconcile logics that have been registered
func ApplyReconcilerHook(cr *argoprojv1alpha1.ArgoCD, i interface{}, hint string) error {
	mutex.Lock()
	defer mutex.Unlock()
	for _, v := range hooks {
		if err := v(cr, i, hint); err != nil {
			return err
		}
	}
	return nil
}

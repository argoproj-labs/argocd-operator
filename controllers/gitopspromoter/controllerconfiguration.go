// Copyright 2026 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitopspromoter

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	promoter "github.com/argoproj-labs/gitops-promoter/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// ReconcilePromoterControllerConfiguration reconciles the Promoter's ControllerConfiguration. It handles creation, updating, and deletion.
func ReconcilePromoterControllerConfiguration(client client.Client, compName string, cr *argoproj.ArgoCD) (*promoter.ControllerConfiguration, error) {
	controllerConfiguration := buildControllerConfiguration(compName, cr)

	allowed := argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace)

	exists := true
	if err := argoutil.FetchObject(client, controllerConfiguration.Namespace, controllerConfiguration.Name, controllerConfiguration); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing controller configuration %s: %v", controllerConfiguration.Name, err)
		}
		exists = false
	}

	if exists {
		if !cr.Spec.Promoter.IsEnabled() || !allowed {
			expectedLabels := buildLabelsForPromoterResources(compName, cr)
			if controllerConfiguration.Labels[common.ArgoCDKeyManagedBy] != expectedLabels[common.ArgoCDKeyManagedBy] {
				return controllerConfiguration, nil
			}
			argoutil.LogResourceDeletion(log, controllerConfiguration, "promoter controller configuration is being deleted due to being disabled")
			if err := client.Delete(context.Background(), controllerConfiguration); err != nil {
				return nil, fmt.Errorf("failed to delete controller configuration %s: %v", controllerConfiguration.Name, err)
			}
			return controllerConfiguration, nil
		}
		return controllerConfiguration, nil
	}

	if !cr.Spec.Promoter.IsEnabled() || !allowed {
		return controllerConfiguration, nil
	}

	argoutil.LogResourceCreation(log, controllerConfiguration)
	if err := client.Create(context.Background(), controllerConfiguration); err != nil {
		return nil, fmt.Errorf("failed to create controller configuration %s: %v", controllerConfiguration.Name, err)
	}
	return controllerConfiguration, nil
}

// buildControllerConfiguration builds the full default ControllerConfiguration
func buildControllerConfiguration(compName string, cr *argoproj.ArgoCD) *promoter.ControllerConfiguration {
	return &promoter.ControllerConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			// At the current moment, the controller fetches a configuration with the hard coded name of promoter-controller-configuration
			// https://github.com/argoproj-labs/gitops-promoter/blob/1f41d319c802c593ac91247ffe30b822680cfdd5/internal/settings/manager.go#L17
			Name:      "promoter-controller-configuration",
			Namespace: cr.Namespace,
			Labels:    buildLabelsForPromoterResources(compName, cr),
		},
		Spec: promoter.ControllerConfigurationSpec{
			PromotionStrategy: promoter.PromotionStrategyConfiguration{
				WorkQueue: buildDefaultWorkQueueSettings(),
			},
			ChangeTransferPolicy: promoter.ChangeTransferPolicyConfiguration{
				WorkQueue: buildDefaultWorkQueueSettings(),
			},
			PullRequest: promoter.PullRequestConfiguration{
				WorkQueue: buildDefaultWorkQueueSettings(),
				Template: promoter.PullRequestTemplate{
					Title:       "Promote {{ with .ChangeTransferPolicy.Spec.ActivePath }}`{{ . }}` {{ end }}({{ trunc 5 .ChangeTransferPolicy.Status.Proposed.Dry.Sha }}) to `{{ .ChangeTransferPolicy.Spec.ActiveBranch }}`",
					Description: "This PR is promoting {{ with .ChangeTransferPolicy.Spec.ActivePath }}`{{ . }}` on {{ end }}the environment branch `{{ .ChangeTransferPolicy.Spec.ActiveBranch }}` from dry sha {{ .ChangeTransferPolicy.Status.Active.Dry.Sha }} to {{ .ChangeTransferPolicy.Status.Proposed.Dry.Sha }}.",
				},
			},
			CommitStatus: promoter.CommitStatusConfiguration{
				WorkQueue: buildDefaultWorkQueueSettings(),
			},
			ArgoCDCommitStatus: promoter.ArgoCDCommitStatusConfiguration{
				WorkQueue:              buildDefaultWorkQueueSettings(),
				WatchLocalApplications: true,
			},
			TimedCommitStatus: promoter.TimedCommitStatusConfiguration{
				WorkQueue: buildDefaultWorkQueueSettings(),
			},
			GitCommitStatus: promoter.GitCommitStatusConfiguration{
				WorkQueue: buildDefaultWorkQueueSettings(),
			},
			WebRequestCommitStatus: promoter.WebRequestCommitStatusConfiguration{
				WorkQueue: buildDefaultWorkQueueSettings(),
			},
			ScheduledCommitStatus: promoter.ScheduledCommitStatusConfiguration{
				WorkQueue: buildDefaultWorkQueueSettings(),
			},
		},
	}
}

// buildDefaultWorkQueueSettings creates the default settings for a WorkQueue
func buildDefaultWorkQueueSettings() promoter.WorkQueue {
	return promoter.WorkQueue{
		MaxConcurrentReconciles: 10,
		RequeueDuration: metav1.Duration{
			Duration: 5 * time.Minute,
		},
		RateLimiter: promoter.RateLimiter{
			MaxOf: []promoter.RateLimiterTypes{
				{
					Bucket: &promoter.Bucket{
						Qps: 10, Bucket: 100,
					},
				},
				{
					FastSlow: &promoter.FastSlow{
						FastDelay: metav1.Duration{
							Duration: 1 * time.Minute,
						},
						SlowDelay: metav1.Duration{
							Duration: 5 * time.Minute,
						},
						MaxFastAttempts: 3,
					},
				},
			},
		},
	}
}

// DeleteControllerConfigurations deletes a list of ControllerConfigurations
func DeleteControllerConfigurations(c client.Client, controllerConfigList *promoter.ControllerConfigurationList) error {
	for _, config := range controllerConfigList.Items {
		argoutil.LogResourceDeletion(log, &config, "cleaning up cluster resources")
		if err := c.Delete(context.TODO(), &config); err != nil {
			return fmt.Errorf("failed to delete ControllerConfiguration %s during cleanup: %w", config.Name, err)
		}
	}
	return nil
}

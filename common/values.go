// Copyright 2020 ArgoCD Operator Developers
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

package common

import "time"

// Argo CD values
const (
	// ArgoCDDuration365Days is a duration representing 365 days.
	ArgoCDDuration365Days = time.Hour * 24 * 365

	// ArgoCDExportStorageBackendAWS is the value for the AWS storage backend.
	ArgoCDExportStorageBackendAWS = "aws"

	// ArgoCDExportStorageBackendAzure is the value for the Azure storage backend.
	ArgoCDExportStorageBackendAzure = "azure"

	// ArgoCDExportStorageBackendGCP is the value for the GCP storage backend.
	ArgoCDExportStorageBackendGCP = "gcp"

	// ArgoCDExportStorageBackendLocal is the value for the local storage backend.
	ArgoCDExportStorageBackendLocal = "local"

	// ArgoCDStatusCompleted is the completed status value.
	ArgoCDStatusCompleted = "Completed"

	// K8sOSLinux is the value for kubernetes.io/os key for linux pods
	K8sOSLinux = "linux"

	// ArgoCDRedisServerTLS is the redis server tls value.
	ArgoCDRedisServerTLS = "argocd-operator-redis-tls"

	// ArgoCDMetrics is the resource metrics key for labels.
	ArgoCDMetrics = "metrics"

	ArgoCDStatusUnknown = "Unknown"

	ArgoCDStatusUnavailable = "Unavailable"

	ArgoCDStatusPending = "Pending"

	ArgoCDStatusRunning = "Running"

	ArgoCDStatusFailed = "Failed"

	ArgoCDStatusAvailable = "Available"

	PrometheusOperator = "prometheus-operator"

	ArgoCDSecretTypeCluster = "cluster"

	// ArgoCDRBACTypeAppSetManagement is the value used when an rbac resource is targeted for applicatonset management
	ArgoCDRBACTypeAppSetManagement = "appset-management"

	// ArgoCDRBACTypeAppManagement is the value used when an rbac resource is targeted for applicaton management
	ArgoCDRBACTypeAppManagement = "app-management"

	// ArgoCDRBACTypeAppManagement is the value used when an rbac resource is targeted for resource management
	ArgoCDRBACTypeResourceMananagement = "resource-management"
)

// general values
const (
	TimeFormatMST                = "01022006-150406-MST"
	TLSCerts                     = "tls-certs"
	CapabilityDropAll            = "ALL"
	VolumeMountPathTLS           = "/app/config/tls"
	VolumeMountPathRepoServerTLS = "/app/config/reposerver/tls"
	WorkingDirApp                = "/app"
	Webhook                      = "webhook"
	SSHKnownHosts                = "ssh-known-hosts"
	VolumeMountPathSSH           = "/app/config/ssh"
	GPGKeys                      = "gpg-keys"
	VolumeMountPathGPG           = "/app/config/gpg/source"
	GPGKeyRing                   = "gpg-keyring"
	VolumeMountPathGPGKeyring    = "/app/config/gpg/keys"
	VolumeTmp                    = "tmp"
	VolumeMountPathTmp           = "/tmp"
)

// API group versions and resource kinds
const (
	APIVersionV1          = "v1"
	APIGroupVersionAppsV1 = "apps/v1"
	APIGroupVersionRbacV1 = "rbac.authorization.k8s.io/v1"

	DeploymentKind     = "Deployment"
	RoleKind           = "Role"
	RoleBindingKind    = "RoleBinding"
	ConfigMapKind      = "ConfigMap"
	SecretKind         = "Secret"
	ServiceKind        = "Service"
	ServiceAccountKind = "ServiceAccount"
	ArgoCDKind         = "ArgoCD"
	ClusterRoleKind    = "ClusterRole"
)

// Commnds
const (
	LogLevelCmd     = "--loglevel"
	LogFormatCmd    = "--logformat"
	UidEntryPointSh = "uid_entrypoint.sh"
)

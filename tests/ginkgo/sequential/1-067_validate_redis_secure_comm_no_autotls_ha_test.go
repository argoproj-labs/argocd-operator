/*
Copyright 2025.

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

package sequential

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deplFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	nodeFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/node"
	osFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/os"
	statefulsetFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/statefulset"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-067_validate_redis_secure_comm_no_autotls_ha", func() {

		var (
			k8sClient   client.Client
			ctx         context.Context
			ns          *corev1.Namespace
			cleanupFunc func()
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			// - Was previously in parallel, moved to sequential due to it requiring a large resource (memory/cpu) commitment for pods

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			defer cleanupFunc()
			fixture.OutputDebugOnFail(ns)
		})

		It("ensures that redis HA can be enabled with tls with generated certificate", func() {
			By("verifying we are running on a cluster with at least 3 nodes. This is required for Redis HA")
			nodeFixture.ExpectHasAtLeastXNodes(3)

			// Note: Redis HA requires a cluster which contains multiple nodes

			By("creating simple namespace-scoped Argo CD instance")
			ns, cleanupFunc = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					HA: argov1beta1api.ArgoCDHASpec{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			expectComponentsAreRunning := func() {

				// In BeAvailable() we wait 15 seconds for ArgoCD CR to be reconciled, this SHOULD be enough time.

				By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
				Eventually(argoCD, "10m", "10s").Should(argocdFixture.BeAvailable())

				deploymentsShouldExist := []string{"argocd-redis-ha-haproxy", "argocd-server", "argocd-repo-server"}
				for _, depl := range deploymentsShouldExist {
					replicas := 1
					if depl == "argocd-redis-ha-haproxy" {
						replicas = 3
					}

					depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: depl, Namespace: ns.Name}}
					Eventually(depl).Should(k8sFixture.ExistByName())
					Eventually(depl).Should(deplFixture.HaveReadyReplicas(replicas))
				}

				statefulsetsShouldExist := []string{"argocd-redis-ha-server", "argocd-application-controller"}
				for _, ss := range statefulsetsShouldExist {

					replicas := 1
					if ss == "argocd-redis-ha-server" {
						replicas = 3
					}

					statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: ss, Namespace: ns.Name}}
					Eventually(statefulSet).Should(k8sFixture.ExistByName())
					Eventually(statefulSet).Should(statefulsetFixture.HaveReplicas(replicas))
					Eventually(statefulSet).Should(statefulsetFixture.HaveReadyReplicas(replicas))
				}

			}

			expectComponentsAreRunning()

			By("generating a test certificate to use with redis, using openssl")

			redis_crt_File, err := os.CreateTemp("", "redis.crt")
			Expect(err).ToNot(HaveOccurred())

			redis_key_File, err := os.CreateTemp("", "redis.key")
			Expect(err).ToNot(HaveOccurred())

			openssl_test_File, err := os.CreateTemp("", "openssl_test.cnf")
			Expect(err).ToNot(HaveOccurred())

			opensslTestCNFContents := "\n[SAN]\nsubjectAltName=DNS:argocd-redis." + ns.Name + ".svc.cluster.local\n[req]\ndistinguished_name=req"

			err = os.WriteFile(openssl_test_File.Name(), ([]byte)(opensslTestCNFContents), 0666)
			Expect(err).ToNot(HaveOccurred())

			_, err = osFixture.ExecCommandWithOutputParam(false, true, "openssl", "req", "-new", "-x509", "-sha256",
				"-subj", "/C=XX/ST=XX/O=Testing/CN=redis",
				"-reqexts", "SAN",
				"-extensions", "SAN",
				"-config", openssl_test_File.Name(),
				"-keyout", redis_key_File.Name(),
				"-out", redis_crt_File.Name(),
				"-newkey", "rsa:4096",
				"-nodes",
				"-days", "10",
			)
			Expect(err).ToNot(HaveOccurred())

			By("creating argocd-operator-redis-tls secret from that cert")

			_, err = osFixture.ExecCommand("kubectl", "create", "secret", "tls", "argocd-operator-redis-tls", "--key="+redis_key_File.Name(), "--cert="+redis_crt_File.Name(), "-n", ns.Name)
			Expect(err).ToNot(HaveOccurred())

			expectComponentsAreRunning()

			By("adding argo cd label to argocd-operator-redis-tls secret")
			_, err = osFixture.ExecCommand("kubectl", "annotate", "secret", "argocd-operator-redis-tls", "argocds.argoproj.io/name=argocd", "-n", ns.Name)
			Expect(err).ToNot(HaveOccurred())

			expectComponentsAreRunning()

			By("extracting the contents of /data/conf/redis.conf and checking it contains expected values")
			redisConf, err := osFixture.ExecCommandWithOutputParam(false, true, "kubectl", "exec", "-i", "pod/argocd-redis-ha-server-0", "-n", ns.Name, "-c", "redis", "--", "cat", "/data/conf/redis.conf")
			Expect(err).ToNot(HaveOccurred())
			expectedRedisConfig := []string{
				"port 0",
				"tls-port 6379",
				"tls-cert-file /app/config/redis/tls/tls.crt",
				"tls-ca-cert-file /app/config/redis/tls/tls.crt",
				"tls-key-file /app/config/redis/tls/tls.key",
				"tls-replication yes",
				"tls-auth-clients no",
			}
			for _, line := range expectedRedisConfig {
				Expect(redisConf).To(ContainSubstring(line))
			}

			By("extracting the contents of /data/conf/sentinel.conf and checking it contains expected values")
			sentinelConf, err := osFixture.ExecCommandWithOutputParam(false, true, "kubectl", "exec", "-i", "pod/argocd-redis-ha-server-0", "-n", ns.Name, "-c", "redis", "--", "cat", "/data/conf/sentinel.conf")
			Expect(err).ToNot(HaveOccurred())
			expectedSentinelConfig := []string{
				"port 0",
				"tls-port 26379",
				"tls-cert-file \"/app/config/redis/tls/tls.crt\"",
				"tls-ca-cert-file \"/app/config/redis/tls/tls.crt\"",
				"tls-key-file \"/app/config/redis/tls/tls.key\"",
				"tls-replication yes",
				"tls-auth-clients no",
			}
			for _, line := range expectedSentinelConfig {
				Expect(sentinelConf).To(ContainSubstring(line))
			}

			repoServerDepl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-repo-server", Namespace: ns.Name}}
			Eventually(repoServerDepl).Should(k8sFixture.ExistByName())

			By("expecting repo-server to have desired container process command/arguments")
			Expect(repoServerDepl).To(deplFixture.HaveContainerCommandSubstring("uid_entrypoint.sh argocd-repo-server --redis argocd-redis-ha-haproxy."+ns.Name+".svc.cluster.local:6379 --redis-use-tls --redis-ca-certificate /app/config/reposerver/tls/redis/tls.crt --loglevel info --logformat text", 0),
				"TLS .spec.template.spec.containers.command for argocd-repo-server deployment is wrong")

			argocdServerDepl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-server", Namespace: ns.Name}}
			Eventually(argocdServerDepl).Should(k8sFixture.ExistByName())

			By("expecting argocd-server to have desired container process command/arguments")
			Expect(argocdServerDepl).To(deplFixture.HaveContainerCommandSubstring("argocd-server --staticassets /shared/app --dex-server https://argocd-dex-server."+ns.Name+".svc.cluster.local:5556 --repo-server argocd-repo-server."+ns.Name+".svc.cluster.local:8081 --redis argocd-redis-ha-haproxy."+ns.Name+".svc.cluster.local:6379 --redis-use-tls --redis-ca-certificate /app/config/server/tls/redis/tls.crt --loglevel info --logformat text", 0),
				"TLS .spec.template.spec.containers.command for argocd-server deployment is wrong")

			By("expecting application-controller to have desired container process command/arguments")
			applicationControllerSS := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "argocd-application-controller", Namespace: ns.Name}}
			Eventually(applicationControllerSS).Should(k8sFixture.ExistByName())

			Expect(applicationControllerSS).To(statefulsetFixture.HaveContainerCommandSubstring("argocd-application-controller --operation-processors 10 --redis argocd-redis-ha-haproxy."+ns.Name+".svc.cluster.local:6379 --redis-use-tls --redis-ca-certificate /app/config/controller/tls/redis/tls.crt --repo-server argocd-repo-server."+ns.Name+".svc.cluster.local:8081 --status-processors 20 --kubectl-parallelism-limit 10 --loglevel info --logformat text", 0),
				"TLS .spec.template.spec.containers.command for argocd-application-controller statefulsets is wrong")
		})

		It("verify redis credential distribution", func() {
			By("verifying we are running on a cluster with at least 3 nodes. This is required for Redis HA")
			nodeFixture.ExpectHasAtLeastXNodes(3)

			By("creating simple Argo CD instance")
			ns, cleanupFunc = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					HA: argov1beta1api.ArgoCDHASpec{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verify redis creds are correctly passed to pods")
			const expectedMsg = "Loading Redis credentials from mounted directory: /app/config/redis-auth/"
			expectedComponents := []string{
				"statefulset/" + argoCD.Name + "-" + "application-controller",
				"deployment/" + argoCD.Name + "-" + "repo-server",
				"deployment/" + argoCD.Name + "-" + "server",
			}
			for _, component := range expectedComponents {
				logOutput, err := osFixture.ExecCommandWithOutputParam(false, true,
					"kubectl", "logs", component, "-n", ns.Name,
				)
				Expect(err).ToNot(HaveOccurred(), "Output: "+logOutput)
				Expect(logOutput).To(ContainSubstring(expectedMsg))
				// Some logs how redis disconnect manifests
				Expect(logOutput).ToNot(ContainSubstring("manifest cache error"))
				Expect(logOutput).ToNot(ContainSubstring("WRONGPASS"))
			}

			By("verifying redis password is correct")
			redisInitialSecret := &corev1.Secret{}
			redisPwdSecretKey := client.ObjectKey{
				Name:      argoutil.GetSecretNameWithSuffix(argoCD, "redis-initial-password"),
				Namespace: ns.Name,
			}
			Expect(k8sClient.Get(ctx, redisPwdSecretKey, redisInitialSecret)).Should(Succeed())
			expectedRedisPwd := string(redisInitialSecret.Data["admin.password"])
			Expect(expectedRedisPwd).ShouldNot(Equal(""))

			redisPingOut, err := osFixture.ExecCommandWithOutputParam(false, false,
				"kubectl", "exec", "-n", ns.Name, "-c", "redis", "pod/argocd-redis-ha-server-0", "--",
				"redis-cli", "-a", expectedRedisPwd, "--no-auth-warning", "ping",
			)

			Expect(err).ToNot(HaveOccurred(), "Output: "+redisPingOut)
			Expect(redisPingOut).To(ContainSubstring("PONG"))
		})
	})
})

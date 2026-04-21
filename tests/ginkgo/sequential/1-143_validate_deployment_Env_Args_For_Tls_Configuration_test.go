/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
*/

package sequential

import (
	"context"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	osFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/os"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("Validate Deployment Env Args For TLS Configuration", func() {
	const (
		argocdNamespace    = "test-tls-argocd"
		argocdInstanceName = "example-argocd"
	)
	var (
		c   client.Client
		ctx context.Context
	)
	BeforeEach(func() {
		fixture.EnsureSequentialCleanSlate()
		c, _ = utils.GetE2ETestKubeClient()
		ctx = context.Background()
	})
	BeforeEach(func() {
		if fixture.EnvLocalRun() {
			Skip("This test is known not to work when running gitops operator locally")
		}
	})
	// --- Helper: Extract TLS values from args ---
	getTLSValues := func(args []string) (min string, max string, hasMin bool, hasMax bool, hasCiphers bool, ciphers string) {
		for i := 0; i < len(args); i++ {
			arg := args[i]
			// handle --tlsminversion <value>
			if arg == "--tlsminversion" {
				hasMin = true
				if i+1 < len(args) {
					min = args[i+1]
				}
			}
			// handle --tlsmaxversion <value>
			if arg == "--tlsmaxversion" {
				hasMax = true
				if i+1 < len(args) {
					max = args[i+1]
				}
			}
			if arg == "--tlsciphers" {
				hasCiphers = true
				if i+1 < len(args) {
					ciphers = args[i+1]
				}
			}
			// handle --tlsminversion=value
			if len(arg) > len("--tlsminversion=") && arg[:len("--tlsminversion=")] == "--tlsminversion=" {
				hasMin = true
				min = arg[len("--tlsminversion="):]
			}
			// handle --tlsmaxversion=value
			if len(arg) > len("--tlsmaxversion=") && arg[:len("--tlsmaxversion=")] == "--tlsmaxversion=" {
				hasMax = true
				max = arg[len("--tlsmaxversion="):]
			}
			if len(arg) > len("--tlsciphers=") && arg[:len("--tlsciphers=")] == "--tlsciphers=" {
				hasCiphers = true
				ciphers = arg[len("--tlsciphers="):]
			}
		}
		return
	}

	Context("When the ArgoCD instance is created with default TLS settings", func() {
		It("should validate default TLS values and updates on RepoServer, Server and Redis Deployments", func() {
			By("creating namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: argocdNamespace,
				},
			}
			Expect(c.Create(ctx, ns)).To(Succeed())

			By("generating a test certificate to use with redis, using openssl")
			redis_crt_File, err := os.CreateTemp("", "redis.crt")
			Expect(err).ToNot(HaveOccurred())

			redis_key_File, err := os.CreateTemp("", "redis.key")
			Expect(err).ToNot(HaveOccurred())

			openssl_test_File, err := os.CreateTemp("", "openssl_test.cnf")
			Expect(err).ToNot(HaveOccurred())

			opensslTestCNFContents := "\n[SAN]\nsubjectAltName=DNS:argocd-redis." + argocdNamespace + ".svc.cluster.local\n[req]\ndistinguished_name=req"

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
			_, err = osFixture.ExecCommand("kubectl", "create", "secret", "tls", "argocd-operator-redis-tls", "--key="+redis_key_File.Name(), "--cert="+redis_crt_File.Name(), "-n", argocdNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("adding argo cd label to argocd-operator-redis-tls secret")
			_, err = osFixture.ExecCommand("kubectl", "annotate", "secret", "argocd-operator-redis-tls", "argocds.argoproj.io/name=argocd", "-n", argocdNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("creating ArgoCD instance")
			argo := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argocdInstanceName,
					Namespace: argocdNamespace,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					ImageUpdater: argov1beta1api.ArgoCDImageUpdaterSpec{
						Enabled: true,
					},
				},
			}
			Expect(c.Create(ctx, argo)).To(Succeed())

			By("waiting for ArgoCD to be available")
			Eventually(func() error {
				return c.Get(ctx, types.NamespacedName{Name: argocdInstanceName, Namespace: argocdNamespace}, &argov1beta1api.ArgoCD{})
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
			defer func() {
				By("cleaning up resources")
				_ = c.Delete(ctx, argo)
				_ = c.Delete(ctx, ns)
				os.Remove(redis_crt_File.Name())
				os.Remove(redis_key_File.Name())
				os.Remove(openssl_test_File.Name())
			}()
			coreDeployments := []string{
				"example-argocd-server",
				"example-argocd-repo-server",
				"example-argocd-argocd-image-updater-controller",
			}
			// --- Validate default TLS values ---
			for _, deploymentName := range coreDeployments {
				deployment := &appsv1.Deployment{}
				By("waiting for deployment " + deploymentName)
				Eventually(func() error {
					return c.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: argocdNamespace}, deployment)
				}, 2*time.Minute, 2*time.Second).Should(Succeed())

				By("validating default TLS args in " + deploymentName)
				Eventually(func() bool {
					if err := c.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: argocdNamespace}, deployment); err != nil {
						return false
					}
					for _, container := range deployment.Spec.Template.Spec.Containers {
						min, max, hasMin, hasMax, _, ciphers := getTLSValues(container.Args)
						if !hasMin || !hasMax {
							continue
						}
						if min != "1.3" {
							GinkgoWriter.Printf("%s: expected tlsminversion=1.3, got %s\n", deploymentName, min)
							return false
						}
						if max != "1.3" {
							GinkgoWriter.Printf("%s: expected tlsmaxversion=1.3, got %s\n", deploymentName, max)
							return false
						}
						if ciphers != "" {
							GinkgoWriter.Printf("%s: expected empty tlsciphers, got %s\n", deploymentName, ciphers)
							return false
						}
						GinkgoWriter.Printf("%s default TLS OK: min=%s max=%s\n", deploymentName, min, max)
						return true
					}
					return false
				}, 60*time.Second, 2*time.Second).Should(BeTrue())
			}
			By("validating default TLS args in Redis deployment")
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				if err := c.Get(ctx, types.NamespacedName{Name: "example-argocd-redis", Namespace: argocdNamespace}, deployment); err != nil {
					return false
				}
				if len(deployment.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				args := deployment.Spec.Template.Spec.Containers[0].Args
				var tlsProtocols string
				var tlsCipherSuites string
				var tlsCiphers string
				hasProtocols := false
				hasCiphers := false
				hasCiphersTLS2 := false
				for i := 0; i < len(args); i++ {
					arg := args[i]
					// --- Handle "--tls-protocols <value>"
					if arg == "--tls-protocols" {
						hasProtocols = true
						if i+1 < len(args) {
							tlsProtocols = args[i+1]
						}
					}
					// --- Handle "--tls-ciphersuites <value>"
					if arg == "--tls-ciphersuites" {
						hasCiphers = true
						if i+1 < len(args) {
							tlsCipherSuites = args[i+1]
						}
					}
					if arg == "--tls-ciphers" {
						hasCiphersTLS2 = true
						if i+1 < len(args) {
							tlsCiphers = args[i+1]
						}
					}
				}

				// --- Print results (always helpful in debugging)
				if hasCiphers || tlsCipherSuites != "" {
					GinkgoWriter.Printf("  --tls-ciphersuites=%q\n should be empty", tlsCipherSuites)
					return false
				}
				if hasCiphersTLS2 || tlsCiphers != "" {
					GinkgoWriter.Printf("  --tls-ciphers=%q\n should be empty", tlsCiphers)
					return false
				}

				if !hasProtocols || tlsProtocols != "TLSv1.3" {
					GinkgoWriter.Printf("%s: expected --tls-protocols=TLSv1.3, got %s\n", deployment.Name, tlsProtocols)
					return false
				}
				GinkgoWriter.Printf("%s TLS args protocol value: %s\n", deployment.Name, tlsProtocols)
				GinkgoWriter.Printf("%s TLS args ciphersuites value: %s\n", deployment.Name, tlsCipherSuites)
				GinkgoWriter.Printf("%s TLS args ciphers value: %s\n", deployment.Name, tlsCiphers)
				return true
			}, 60*time.Second, 2*time.Second).Should(BeTrue())

			// --- Update TLS config ---
			By("updating TLS config in ArgoCD CR For RepoServer, Server and Redis")
			Expect(c.Get(ctx, types.NamespacedName{Name: argocdInstanceName, Namespace: argocdNamespace}, argo)).To(Succeed())
			argo.Spec.Repo.TlsConfig = &argov1beta1api.ArgoCDTlsConfig{
				MinVersion: "1.2",
				MaxVersion: "1.3",
			}
			argo.Spec.Server.TlsConfig = &argov1beta1api.ArgoCDTlsConfig{
				MinVersion: "1.2",
				MaxVersion: "1.3",
			}
			argo.Spec.Redis.TlsConfig = &argov1beta1api.ArgoCDTlsConfig{
				MinVersion: "1.1",
				MaxVersion: "1.3",
			}
			argo.Spec.ImageUpdater.TlsConfig = &argov1beta1api.ArgoCDTlsConfig{
				MinVersion: "1.2",
				MaxVersion: "1.3",
			}
			Expect(c.Update(ctx, argo)).To(Succeed())
			time.Sleep(5 * time.Second)
			// --- Validate updated TLS values ---
			By("validating updated TLS args For RepoServer and Server")
			Eventually(func() bool {
				for _, deploymentName := range coreDeployments {
					deployment := &appsv1.Deployment{}
					if err := c.Get(ctx,
						types.NamespacedName{Name: deploymentName, Namespace: argocdNamespace}, deployment); err != nil {
						return false
					}
					valid := false
					for _, container := range deployment.Spec.Template.Spec.Containers {
						min, max, hasMin, hasMax, _, _ := getTLSValues(container.Args)
						if !hasMin || !hasMax {
							continue
						}
						if min != "1.2" {
							GinkgoWriter.Printf("%s: expected tlsminversion=1.2, got %s\n", deploymentName, min)
							return false
						}
						if max != "1.3" {
							GinkgoWriter.Printf("%s: expected tlsmaxversion=1.3, got %s\n", deploymentName, max)
							return false
						}
						GinkgoWriter.Printf("%s updated TLS OK: min=%s max=%s\n", deploymentName, min, max)
						valid = true
					}
					if !valid {
						return false
					}
				}
				return true
			}, 60*time.Second, 2*time.Second).Should(BeTrue(), "all deployments should have updated TLS configuration")

			By("Validating Updated TLS args in Redis deployment")
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				if err := c.Get(ctx, types.NamespacedName{Name: "example-argocd-redis", Namespace: argocdNamespace}, deployment); err != nil {
					return false
				}
				if len(deployment.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				args := deployment.Spec.Template.Spec.Containers[0].Args
				var tlsProtocols string
				var tlsCipherSuites string
				var tlsCiphers string
				hasProtocols := false
				hasCiphers := false
				hasCiphersTLS2 := false
				for i := 0; i < len(args); i++ {
					arg := args[i]
					// --- Handle "--tls-protocols <value>"
					if arg == "--tls-protocols" {
						hasProtocols = true
						if i+1 < len(args) {
							tlsProtocols = args[i+1]
						}
					}
					// --- Handle "--tls-ciphersuites <value>"
					if arg == "--tls-ciphersuites" {
						hasCiphers = true
						if i+1 < len(args) {
							tlsCipherSuites = args[i+1]
						}
					}
					// --- Handle "--tls-ciphers <value>"
					if arg == "--tls-ciphers" {
						hasCiphersTLS2 = true
						if i+1 < len(args) {
							tlsCiphers = args[i+1]
						}
					}
				}

				// --- Print results (always helpful in debugging)
				if hasCiphers || tlsCipherSuites != "" {
					GinkgoWriter.Printf("  --tls-ciphersuites=%q\n should be empty", tlsCipherSuites)
					return false
				}
				if hasCiphersTLS2 || tlsCiphers != "" {
					GinkgoWriter.Printf("  --tls-ciphers=%q\n should be empty", tlsCiphers)
					return false
				}

				if !hasProtocols || tlsProtocols != "TLSv1.1 TLSv1.2 TLSv1.3" {
					GinkgoWriter.Printf("%s: expected --tls-protocols=TLSv1.1 TLSv1.2 TLSv1.3, got %s\n", deployment.Name, tlsProtocols)
					return false
				}
				GinkgoWriter.Printf("%s TLS args protocol value: %s\n", deployment.Name, tlsProtocols)
				GinkgoWriter.Printf("%s TLS args ciphersuites value: %s\n", deployment.Name, tlsCipherSuites)
				GinkgoWriter.Printf("%s TLS args ciphers value: %s\n", deployment.Name, tlsCiphers)
				return true
			}, 60*time.Second, 2*time.Second).Should(BeTrue())

			By("Update single cipherSuites for RepoServer, Server and Redis Deployments")
			Expect(c.Get(ctx, types.NamespacedName{Name: argocdInstanceName, Namespace: argocdNamespace}, argo)).To(Succeed())
			argo.Spec.Repo.TlsConfig.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"}
			argo.Spec.Server.TlsConfig.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"}
			argo.Spec.ImageUpdater.TlsConfig.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"}
			argo.Spec.Redis.TlsConfig = &argov1beta1api.ArgoCDTlsConfig{
				MinVersion:   "1.2",
				MaxVersion:   "1.3",
				CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
			}
			Expect(c.Update(ctx, argo)).To(Succeed())

			time.Sleep(5 * time.Second)
			// --- Validate updated TLS values ---
			By("validating updated TLS single CipherSuites For RepoServer and Server")
			Eventually(func() bool {
				for _, deploymentName := range coreDeployments {
					deployment := &appsv1.Deployment{}
					if err := c.Get(ctx,
						types.NamespacedName{Name: deploymentName, Namespace: argocdNamespace}, deployment); err != nil {
						return false
					}
					valid := false
					for _, container := range deployment.Spec.Template.Spec.Containers {
						min, max, hasMin, hasMax, hasCiphers, ciphers := getTLSValues(container.Args)
						if !hasMin || !hasMax || !hasCiphers {
							continue
						}
						if min != "1.2" {
							GinkgoWriter.Printf("%s: expected tlsminversion=1.2, got %s\n", deploymentName, min)
							return false
						}
						if max != "1.3" {
							GinkgoWriter.Printf("%s: expected tlsmaxversion=1.3, got %s\n", deploymentName, max)
							return false
						}
						if ciphers != "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384" {
							GinkgoWriter.Printf("%s: expected tlsciphers=TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, got %s\n", deploymentName, ciphers)
							return false
						}
						GinkgoWriter.Printf("%s updated TLS OK: min=%s max=%s\n ciphers=%s\n", deploymentName, min, max, ciphers)
						valid = true
					}
					if !valid {
						return false
					}
				}
				return true
			}, 60*time.Second, 2*time.Second).Should(BeTrue(), "all deployments should have updated TLS configuration")

			By("validating TLS single CipherSuite in Redis deployment")
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				if err := c.Get(ctx, types.NamespacedName{Name: "example-argocd-redis", Namespace: argocdNamespace}, deployment); err != nil {
					return false
				}
				if len(deployment.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				args := deployment.Spec.Template.Spec.Containers[0].Args
				var tlsProtocols string
				var tlsCipherSuites string
				var tlsCiphers string
				hasProtocols := false
				hasCiphers := false
				hasCiphersTLS2 := false
				for i := 0; i < len(args); i++ {
					arg := args[i]
					// --- Handle "--tls-protocols <value>"
					if arg == "--tls-protocols" {
						hasProtocols = true
						if i+1 < len(args) {
							tlsProtocols = args[i+1]
						}
					}
					// --- Handle "--tls-ciphersuites <value>"
					if arg == "--tls-ciphersuites" {
						hasCiphers = true
						if i+1 < len(args) {
							tlsCipherSuites = args[i+1]
						}
					}

					// --- Handle "--tls-ciphers <value>"
					if arg == "--tls-ciphers" {
						hasCiphersTLS2 = true
						if i+1 < len(args) {
							tlsCiphers = args[i+1]
						}
					}
				}

				// --- Print results (always helpful in debugging)
				if hasCiphers && tlsCipherSuites != "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384" {
					GinkgoWriter.Printf("  --tls-ciphersuites=%q\n should be TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, but got %q\n", tlsCipherSuites, tlsCipherSuites)
					return false
				}
				if hasCiphersTLS2 && tlsCiphers != "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384" {
					GinkgoWriter.Printf("  --tls-ciphers=%q\n should be TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, but got %q\n", tlsCiphers, tlsCiphers)
					return false
				}
				if !hasProtocols || tlsProtocols != "TLSv1.2 TLSv1.3" {
					GinkgoWriter.Printf("%s: expected --tls-protocols=TLSv1.2 TLSv1.3, got %s\n", deployment.Name, tlsProtocols)
					return false
				}
				GinkgoWriter.Printf("%s TLS args protocol value: %s\n", deployment.Name, tlsProtocols)
				GinkgoWriter.Printf("%s TLS args ciphersuites value: %s\n", deployment.Name, tlsCipherSuites)
				GinkgoWriter.Printf("%s TLS args ciphers value: %s\n", deployment.Name, tlsCiphers)
				return true
			}, 60*time.Second, 2*time.Second).Should(BeTrue())

			By("Update Two cipherSuites for RepoServer, Server and Redis Deployments")
			Expect(c.Get(ctx, types.NamespacedName{Name: argocdInstanceName, Namespace: argocdNamespace}, argo)).To(Succeed())
			argo.Spec.Repo.TlsConfig.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}
			argo.Spec.Server.TlsConfig.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}
			argo.Spec.Redis.TlsConfig.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}
			argo.Spec.ImageUpdater.TlsConfig.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}
			Expect(c.Update(ctx, argo)).To(Succeed())

			time.Sleep(5 * time.Second)
			// --- Validate updated TLS values ---
			By("validating updated TLS double CipherSuites For RepoServer and Server")
			Eventually(func() bool {
				for _, deploymentName := range coreDeployments {
					deployment := &appsv1.Deployment{}
					if err := c.Get(ctx,
						types.NamespacedName{Name: deploymentName, Namespace: argocdNamespace}, deployment); err != nil {
						return false
					}
					valid := false
					for _, container := range deployment.Spec.Template.Spec.Containers {
						min, max, _, _, _, ciphers := getTLSValues(container.Args)
						if min != "1.2" {
							GinkgoWriter.Printf("%s: expected tlsminversion=1.2, got %s\n", deploymentName, min)
							return false
						}
						if max != "1.3" {
							GinkgoWriter.Printf("%s: expected tlsmaxversion=1.3, got %s\n", deploymentName, max)
							return false
						}
						if ciphers != "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" {
							GinkgoWriter.Printf("%s: expected tlsciphers=TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, got %s\n", deploymentName, ciphers)
							return false
						}
						GinkgoWriter.Printf("%s updated TLS OK: min=%s max=%s\n ciphers=%s\n", deploymentName, min, max, ciphers)
						valid = true
					}
					if !valid {
						return false
					}
				}
				return true
			}, 60*time.Second, 2*time.Second).Should(BeTrue(), "all deployments should have updated TLS configuration")

			By("validating TLS double CipherSuite in Redis deployment")
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				if err := c.Get(ctx, types.NamespacedName{Name: "example-argocd-redis", Namespace: argocdNamespace}, deployment); err != nil {
					return false
				}
				if len(deployment.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				args := deployment.Spec.Template.Spec.Containers[0].Args
				var tlsProtocols string
				var tlsCipherSuites string
				var tlsCiphers string
				hasProtocols := false
				hasCiphers := false
				hasCiphersTLS2 := false
				for i := 0; i < len(args); i++ {
					arg := args[i]
					// --- Handle "--tls-protocols <value>"
					if arg == "--tls-protocols" {
						hasProtocols = true
						if i+1 < len(args) {
							tlsProtocols = args[i+1]
						}
					}
					// --- Handle "--tls-ciphersuites <value>"
					if arg == "--tls-ciphersuites" {
						hasCiphers = true
						if i+1 < len(args) {
							tlsCipherSuites = args[i+1]
						}
					}

					if arg == "--tls-ciphers" {
						hasCiphersTLS2 = true
						if i+1 < len(args) {
							tlsCiphers = args[i+1]
						}
					}
				}

				// --- Print results (always helpful in debugging)
				if hasCiphers && tlsCipherSuites != "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" {
					GinkgoWriter.Printf("  --tls-ciphersuites=%q\n should be TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, but got %q\n", tlsCipherSuites, tlsCipherSuites)
					return false
				}

				if hasCiphersTLS2 && tlsCiphers != "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" {
					GinkgoWriter.Printf("  --tls-ciphers=%q\n should be TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, but got %q\n", tlsCiphers, tlsCiphers)
					return false
				}

				if !hasProtocols || tlsProtocols != "TLSv1.2 TLSv1.3" {
					GinkgoWriter.Printf("%s: expected --tls-protocols=TLSv1.2 TLSv1.3, got %s\n", deployment.Name, tlsProtocols)
					return false
				}
				GinkgoWriter.Printf("%s TLS args protocol value: %s\n", deployment.Name, tlsProtocols)
				GinkgoWriter.Printf("%s TLS args ciphersuites value: %s\n", deployment.Name, tlsCipherSuites)
				GinkgoWriter.Printf("%s TLS args ciphers value: %s\n", deployment.Name, tlsCiphers)
				return true
			}, 60*time.Second, 2*time.Second).Should(BeTrue())

			By("Check The deployments doesnt have invalid values and check the argocd CR status for Error")
			Expect(c.Get(ctx, types.NamespacedName{Name: argocdInstanceName, Namespace: argocdNamespace}, argo)).To(Succeed())
			argo.Spec.Repo.TlsConfig.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}
			argo.Spec.Server.TlsConfig.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}
			argo.Spec.Redis.TlsConfig.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}
			argo.Spec.ImageUpdater.TlsConfig.CipherSuites = []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"}
			Expect(c.Update(ctx, argo)).To(Succeed())
			time.Sleep(5 * time.Second)
			Eventually(func() *metav1.Condition {
				err := c.Get(ctx, types.NamespacedName{Name: argocdInstanceName, Namespace: argocdNamespace}, argo)
				if err != nil {
					return nil
				}
				for _, cond := range argo.Status.Conditions {
					if cond.Reason == "ErrorOccurred" && cond.Message == "invalid TLS configuration: unsupported cipher suite: TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" {
						return &cond
					}
				}
				return nil
			}, 30*time.Second, 1*time.Second).ShouldNot(BeNil(), "Expected TLS validation error condition not found")

			// --- Validate updated TLS values ---
			By("validating updated TLS invalid double CipherSuites For RepoServer, Server and Redis")
			Eventually(func() bool {
				for _, deploymentName := range coreDeployments {
					deployment := &appsv1.Deployment{}
					if err := c.Get(ctx,
						types.NamespacedName{Name: deploymentName, Namespace: argocdNamespace}, deployment); err != nil {
						return false
					}
					valid := false
					for _, container := range deployment.Spec.Template.Spec.Containers {
						min, max, _, _, _, ciphers := getTLSValues(container.Args)
						if min != "1.2" {
							GinkgoWriter.Printf("%s: expected tlsminversion=1.2, got %s\n", deploymentName, min)
							return false
						}
						if max != "1.3" {
							GinkgoWriter.Printf("%s: expected tlsmaxversion=1.3, got %s\n", deploymentName, max)
							return false
						}
						if ciphers != "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" {
							GinkgoWriter.Printf("%s: expected tlsciphers=TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, got %s\n", deploymentName, ciphers)
							return false
						}
						GinkgoWriter.Printf("%s not updated TLS invalid values OK: min=%s max=%s\n ciphers=%s\n", deploymentName, min, max, ciphers)
						valid = true
					}
					if !valid {
						return false
					}
				}
				return true
			}, 60*time.Second, 2*time.Second).Should(BeTrue(), "all deployments should have updated TLS configuration")

			By("validating TLS double CipherSuite in Redis deployment")
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				if err := c.Get(ctx, types.NamespacedName{Name: "example-argocd-redis", Namespace: argocdNamespace}, deployment); err != nil {
					return false
				}
				if len(deployment.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				args := deployment.Spec.Template.Spec.Containers[0].Args
				var tlsProtocols string
				var tlsCipherSuites string
				var tlsCiphers string
				hasProtocols := false
				hasCiphers := false
				hasCiphersTLS2 := false
				for i := 0; i < len(args); i++ {
					arg := args[i]
					// --- Handle "--tls-protocols <value>"
					if arg == "--tls-protocols" {
						hasProtocols = true
						if i+1 < len(args) {
							tlsProtocols = args[i+1]
						}
					}
					// --- Handle "--tls-ciphersuites <value>"
					if arg == "--tls-ciphersuites" {
						hasCiphers = true
						if i+1 < len(args) {
							tlsCipherSuites = args[i+1]
						}
					}

					if arg == "--tls-ciphers" {
						hasCiphersTLS2 = true
						if i+1 < len(args) {
							tlsCiphers = args[i+1]
						}
					}

				}

				// --- Print results (always helpful in debugging)
				if hasCiphers && tlsCipherSuites != "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" {
					GinkgoWriter.Printf("  --tls-ciphersuites=%q\n should be TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, but got %q\n", tlsCipherSuites, tlsCipherSuites)
					return false
				}

				if hasCiphersTLS2 && tlsCiphers != "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" {
					GinkgoWriter.Printf("  --tls-ciphers=%q\n should be TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, but got %q\n", tlsCiphers, tlsCiphers)
					return false
				}

				if !hasProtocols || tlsProtocols != "TLSv1.2 TLSv1.3" {
					GinkgoWriter.Printf("%s: expected --tls-protocols=TLSv1.2 TLSv1.3, got %s\n", deployment.Name, tlsProtocols)
					return false
				}
				GinkgoWriter.Print("TLS Invalid  values not populated")
				GinkgoWriter.Printf("%s TLS args protocol value: %s\n", deployment.Name, tlsProtocols)
				GinkgoWriter.Printf("%s TLS args ciphersuites value: %s\n", deployment.Name, tlsCipherSuites)
				GinkgoWriter.Printf("%s TLS args ciphers value: %s\n", deployment.Name, tlsCiphers)
				return true
			}, 60*time.Second, 2*time.Second).Should(BeTrue())
		})
	})
})

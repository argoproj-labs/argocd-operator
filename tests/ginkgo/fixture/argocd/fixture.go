package argocd

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/ginkgo/v2" //nolint:all
	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/gomega" //nolint:all

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	sessionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	matcher "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Update will update an ArgoCD CR. Update will keep trying to update object until it succeeds, or times out.
func Update(obj *argov1beta1api.ArgoCD, modify func(*argov1beta1api.ArgoCD)) {
	k8sClient, _ := utils.GetE2ETestKubeClient()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of the object
		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return err
		}

		modify(obj)

		// Attempt to update the object
		return k8sClient.Update(context.Background(), obj)
	})
	Expect(err).ToNot(HaveOccurred())

	// After we update ArgoCD CR, we should wait a few moments for the operator to reconcile the change.
	// - Ideally, the ArgoCD CR would have a .status field that we could read, that would indicate which resource version/generation had been reconciled.
	// - Sadly, this does not exist, so we instead must use time.Sleep() (for now)
	time.Sleep(7 * time.Second)
}

// BeAvailable waits for Argo CD instance to have .status.phase of 'Available'
func BeAvailable() matcher.GomegaMatcher {
	return BeAvailableWithCustomSleepTime(10 * time.Second)
}

// In most cases, you should probably just use 'BeAvailable'.
func BeAvailableWithCustomSleepTime(sleepTime time.Duration) matcher.GomegaMatcher {

	// Wait X seconds to allow operator to reconcile the ArgoCD CR, before we start checking if it's ready
	// - We do this so that any previous calls to update the ArgoCD CR have been reconciled by the operator, before we wait to see if ArgoCD has become available.
	// - I'm not aware of a way to do this without a sleep statement, but when we have something better we should do that instead.
	time.Sleep(sleepTime)

	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {

		if argocd.Status.Phase != "Available" {
			GinkgoWriter.Println("ArgoCD status is not yet Available")
			return false
		}
		GinkgoWriter.Println("ArgoCD status is now", argocd.Status.Phase)

		return true
	})
}

func HavePhase(phase string) matcher.GomegaMatcher {
	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {
		GinkgoWriter.Println("HavePhase:", "expected:", phase, "/ actual:", argocd.Status.Phase)
		return argocd.Status.Phase == phase
	})
}

func HaveRedisStatus(status string) matcher.GomegaMatcher {
	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {
		GinkgoWriter.Println("HaveRedisStatus:", "expected:", status, "/ actual:", argocd.Status.Redis)
		return argocd.Status.Redis == status
	})
}

func HaveRepoStatus(status string) matcher.GomegaMatcher {
	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {
		GinkgoWriter.Println("HaveRepoStatus:", "expected:", status, "/ actual:", argocd.Status.Repo)
		return argocd.Status.Repo == status
	})
}

func HaveServerStatus(status string) matcher.GomegaMatcher {
	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {
		GinkgoWriter.Println("HaveServerStatus:", "expected:", status, "/ actual:", argocd.Status.Server)
		return argocd.Status.Server == status
	})
}

func HaveApplicationControllerStatus(status string) matcher.GomegaMatcher {
	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {
		GinkgoWriter.Println("HaveApplicationControllerStatus:", "expected:", status, "/ actual:", argocd.Status.ApplicationController)
		return argocd.Status.ApplicationController == status
	})
}

func HaveApplicationSetControllerStatus(status string) matcher.GomegaMatcher {
	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {
		GinkgoWriter.Println("HaveApplicationSetControllerStatus:", "expected:", status, "/ actual:", argocd.Status.ApplicationSetController)
		return argocd.Status.ApplicationSetController == status
	})
}

func HaveNotificationControllerStatus(status string) matcher.GomegaMatcher {
	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {
		GinkgoWriter.Println("HaveNotificationControllerStatus:", "expected:", status, "/ actual:", argocd.Status.NotificationsController)
		return argocd.Status.NotificationsController == status
	})
}

func HaveSSOStatus(status string) matcher.GomegaMatcher {
	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {
		GinkgoWriter.Println("HaveSSOStatus:", "expected:", status, "/ actual:", argocd.Status.SSO)
		return argocd.Status.SSO == status
	})
}

func HaveHost(host string) matcher.GomegaMatcher {
	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {
		GinkgoWriter.Println("HaveHost:", "expected:", host, "/ actual:", argocd.Status.Host)
		return argocd.Status.Host == host
	})
}

func HaveApplicationControllerOperationProcessors(operationProcessors int) matcher.GomegaMatcher {
	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {
		GinkgoWriter.Println("HaveApplicationControllerOperationProcessors:", "Expected:", operationProcessors, "/ actual:", argocd.Spec.Controller.Processors.Operation)
		return int(argocd.Spec.Controller.Processors.Operation) == operationProcessors
	})
}

func HaveCondition(condition metav1.Condition) matcher.GomegaMatcher {
	return fetchArgoCD(func(argocd *argov1beta1api.ArgoCD) bool {

		if len(argocd.Status.Conditions) != 1 {
			GinkgoWriter.Println("HaveCondition: length is zero")
			return false
		}

		instanceCondition := argocd.Status.Conditions[0]

		GinkgoWriter.Println("HaveCondition - Message:", instanceCondition.Message, condition.Message)
		if instanceCondition.Message != condition.Message {
			GinkgoWriter.Println("HaveCondition: message does not match")
			return false
		}

		GinkgoWriter.Println("HaveCondition - Reason:", instanceCondition.Reason, condition.Reason)
		if instanceCondition.Reason != condition.Reason {
			GinkgoWriter.Println("HaveCondition: reason does not match")
			return false
		}

		GinkgoWriter.Println("HaveCondition - Status:", instanceCondition.Status, condition.Status)
		if instanceCondition.Status != condition.Status {
			GinkgoWriter.Println("HaveCondition: status does not match")
			return false
		}

		GinkgoWriter.Println("HaveCondition - Type:", instanceCondition.Type, condition.Type)
		if instanceCondition.Type != condition.Type {
			GinkgoWriter.Println("HaveCondition: type does not match")
			return false
		}

		return true

	})
}

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
func fetchArgoCD(f func(*argov1beta1api.ArgoCD) bool) matcher.GomegaMatcher {

	return WithTransform(func(argocd *argov1beta1api.ArgoCD) bool {

		k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(argocd), argocd)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		return f(argocd)

	}, BeTrue())

}

func RunArgoCDCLI(args ...string) (string, error) {

	cmdArgs := append([]string{"argocd"}, args...)

	GinkgoWriter.Println("executing command", cmdArgs)

	// #nosec G204
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	output, err := cmd.CombinedOutput()
	GinkgoWriter.Println(string(output))

	return string(output), err
}

// CreateArgoCDAPIClient connects to given Argo CD API server endpoint and returns client object for invoking server APIs
func CreateArgoCDAPIClient(ctx context.Context, argoServerEndpoint string, password string) (argocdclient.Client, string, io.Closer, error) {
	var token string

	clientOpts := &argocdclient.ClientOptions{
		ServerAddr: argoServerEndpoint,
		Insecure:   true,
		AuthToken:  password,
	}

	client, err := argocdclient.NewClient(clientOpts)
	if err != nil {
		return nil, "", nil, fmt.Errorf("unable to create new Argo CD client: %v", err)
	}

	closer, sessionClient, err := client.NewSessionClient()
	if err != nil {
		return nil, "", nil, fmt.Errorf("unable to create new Argo CD session client: %v", err)
	}

	sessionResponse, err := sessionClient.Create(ctx, &sessionpkg.SessionCreateRequest{Username: "admin", Password: password})
	if err != nil {
		return nil, "", nil, fmt.Errorf("unable to create invoke session client: %v", err)
	}
	token = sessionResponse.Token

	clientOpts = &argocdclient.ClientOptions{
		ServerAddr: argoServerEndpoint,
		Insecure:   true,
		AuthToken:  token,
	}

	client, err = argocdclient.NewClient(clientOpts)
	if err != nil {
		return nil, "", nil, fmt.Errorf("unable to create new argocd client: %v", err)
	}

	return client, token, closer, nil

}

func GetInitialAdminSecretPassword(argocdCRName string, secretNS string, k8sClient client.Client) string {

	// First look for Secret named (argocd CR name)-cluster
	initialPassword_Secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocdCRName + "-cluster",
			Namespace: secretNS,
		},
	}

	err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(initialPassword_Secret), initialPassword_Secret)
	if err != nil {
		// If not found, next look for argocd-initial-admin-secret
		if apierr.IsNotFound(err) {

			initialPasswordSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-initial-admin-secret",
					Namespace: secretNS,
				},
			}
			Eventually(initialPasswordSecret).Should(k8sFixture.ExistByNameWithClient(k8sClient))
			return string(initialPasswordSecret.Data["password"])
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
	}

	Eventually(initialPassword_Secret).Should(k8sFixture.ExistByNameWithClient(k8sClient))
	return string(initialPassword_Secret.Data["admin.password"])
}

// StreamFromArgoCDEventSourceURL streams event from a given event source URL. Event sources are related to 'server sent events' of HTTP spec, and are used by some Argo CD APIs.
func StreamFromArgoCDEventSourceURL(ctx context.Context, eventSourceAPIURL string, sessionToken string) (chan string, error) {

	msgChan := make(chan string)

	go func() {
		defer GinkgoRecover()

		// connect to URL and read data into msgChan channel (until disconnect)
		// - returns true if the function exited due to cancelled context.
		connect := func(client *http.Client, req *http.Request) bool {
			resp, err := client.Do(req)
			if err != nil {
				GinkgoWriter.Printf("Error performing request: %v", err)

				return strings.Contains(err.Error(), "context canceled")
			}

			defer resp.Body.Close()

			reader := bufio.NewReader(resp.Body)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					GinkgoWriter.Printf("Error reading from stream: %v", err)

					return strings.Contains(err.Error(), "context canceled")
				}

				if strings.HasPrefix(line, "data:") {
					data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
					select {
					case <-ctx.Done():
						GinkgoWriter.Println("Context is complete")
						return true
					default:
						msgChan <- data
					}

				}
			}
		}

		// Loop: if we lose connection, re-establish (unless the context was cancelled)
		for {

			req, err := http.NewRequest("GET", eventSourceAPIURL, nil)
			if err != nil {
				Expect(err).ToNot(HaveOccurred())
				return
			}
			req.Header.Set("Accept", "text/event-stream") // server sent event mime type

			req = req.WithContext(ctx)

			cookie := &http.Cookie{
				Name:  "argocd.token",
				Value: sessionToken, // session token
			}
			req.AddCookie(cookie)

			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- this is test code connecting to a local test server
			}
			client := &http.Client{Transport: tr}

			contextCancelled := connect(client, req)

			if contextCancelled {
				GinkgoWriter.Println("context cancelled on event source stream")
				return
			} else {
				// Wait a few moments on connection fail, to prevent swamping the API with HTTP requests
				time.Sleep(250 * time.Millisecond)
			}
		}

	}()

	return msgChan, nil
}

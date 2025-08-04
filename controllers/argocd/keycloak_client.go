package argocd

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	json "encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

type requester interface {
	Do(req *http.Request) (*http.Response, error)
}

type httpclient struct {
	requester requester
	URL       string
	token     string
}

// Creates a new realm for Keycloak.
func (r *ReconcileArgoCD) createRealm(cfg *keycloakConfig) (string, error) {

	req, err := defaultRequester(cfg.KeycloakServerCert, cfg.VerifyTLS)
	if err != nil {
		return "", err
	}

	// create a new http client.
	h := &httpclient{
		requester: req,
	}

	kSvcName := h.getKeycloakURL(cfg.ArgoNamespace)
	if kSvcName != "" {
		cfg.KeycloakURL = kSvcName
	}

	h.URL = cfg.KeycloakURL

	// login request updates the auth token for httpclient.
	err = h.login(cfg.Username, cfg.Password)
	if err != nil {
		return "", err
	}
	log.Info(fmt.Sprintf("Access Token for keycloak of ArgoCD %s in namespace %s generated successfully",
		cfg.ArgoName, cfg.ArgoNamespace))

	realmConfig, err := r.createRealmConfig(cfg)
	if err != nil {
		return "", err
	}

	status, _ := h.post(realmConfig)

	return status, nil
}

// login requests a new auth token.
func (h *httpclient) login(user, pass string) error {
	form := url.Values{}
	form.Add("username", user)
	form.Add("password", pass)
	form.Add("client_id", "admin-cli")
	form.Add("grant_type", "password")

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s%s", h.URL, authURL),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := h.requester.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	tokenRes := &TokenResponse{}
	err = json.Unmarshal(body, tokenRes)
	if err != nil {
		return err
	}

	if tokenRes.Error != "" {
		return err
	}

	h.token = tokenRes.AccessToken

	return nil
}

// Post the updated realm configuration to keycloak realm API.
func (h *httpclient) post(realmConfig []byte) (string, error) {
	request, err := http.NewRequest("POST",
		fmt.Sprintf("%s%s", h.URL, realmURL),
		bytes.NewBuffer(realmConfig))

	if err != nil {
		return "", err
	}

	// set headers.
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", h.token))

	response, err := h.requester.Do(request)
	if err != nil {
		return "", err
	}

	return response.Status, nil
}

// defaultRequester returns a default client for requesting http endpoints.
func defaultRequester(serverCert []byte, verifyTLS bool) (requester, error) {
	tlsConfig, err := createTLSConfig(serverCert, verifyTLS)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	c := &http.Client{Transport: transport}
	return c, nil
}

// createTLSConfig constructs and returns a TLS Config with a root CA read
// from the serverCert param if present, or a permissive config which
// is insecure otherwise.
// An Insecure config is returned also when .spec.SSO.verifyTLS is set to false.
func createTLSConfig(serverCert []byte, verifyTLS bool) (*tls.Config, error) {
	if serverCert == nil || !verifyTLS {
		return &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, nil // #nosec G402
	}

	rootCAPool := x509.NewCertPool()
	if ok := rootCAPool.AppendCertsFromPEM(serverCert); !ok {
		return nil, errors.Errorf("unable to successfully load certificate")
	}
	return &tls.Config{RootCAs: rootCAPool, MinVersion: tls.VersionTLS12}, nil
}

// Get Keycloak URL.
func (h *httpclient) getKeycloakURL(ns string) string {

	svc := fmt.Sprintf("https://%s.%s.svc.cluster.local:%s", defaultKeycloakIdentifier, ns, "8443")
	// At normal conditions, Keycloak should be accessible via the service name. However, there are some corner cases (like
	// operator running locally during development or services being inaccessible due to network policies) which requires
	// use of externalURL.
	err := h.validateKeycloakURL(svc)
	if err != nil {
		return ""
	}

	return svc
}

func (h *httpclient) validateKeycloakURL(URL string) error {
	req, err := http.NewRequest(
		"GET",
		URL,
		nil,
	)
	if err != nil {
		return err
	}

	res, err := h.requester.Do(req)
	if err != nil {
		log.Info("Cannot access keycloak with Internal service name, trying keycloak Route URL")
		return fmt.Errorf("cannot access keycloak with Internal service name, trying keycloak Route URL. error: %w", err)
	}
	_ = res.Body.Close()
	return nil
}

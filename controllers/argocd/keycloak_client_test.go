package argocd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"encoding/pem"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestKeycloak_testRealmCreation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, realmURL, req.URL.Path)
		w.WriteHeader(201)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	h := &httpclient{
		requester: server.Client(),
		URL:       server.URL,
		token:     "dummy",
	}

	a := makeTestArgoCD()
	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	data := &keycloakConfig{}
	realm, _ := r.createRealmConfig(data)

	_, err := h.post(realm)
	assert.NoError(t, err)
}

func TestKeycloak_testLogin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, authURL, req.URL.Path)
		assert.Equal(t, req.Method, http.MethodPost)

		response := TokenResponse{
			AccessToken: "dummy",
		}

		json, err := jsoniter.Marshal(response)
		assert.NoError(t, err)

		size, err := w.Write(json)
		assert.NoError(t, err)
		assert.Equal(t, size, len(json))

		w.WriteHeader(204)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	h := &httpclient{
		requester: server.Client(),
		URL:       server.URL,
		token:     "not set",
	}

	err := h.login("dummy", "dummy")

	assert.NoError(t, err)
	assert.Equal(t, h.token, "dummy")
}

func TestClient_useKeycloakServerCertificate(t *testing.T) {
	var insecure bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, err := w.Write([]byte("dummy"))
		if err != nil {
			t.Errorf("dummy write failed with error %v", err)
		}
	})
	ts := httptest.NewTLSServer(handler)
	defer ts.Close()

	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})

	requester, err := defaultRequester(pemCert, true)
	assert.NoError(t, err)
	httpClient, ok := requester.(*http.Client)
	assert.Equal(t, true, ok)
	assert.Equal(t, httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify, insecure)

	request, err := http.NewRequest("GET", ts.URL, nil)
	assert.NoError(t, err)
	resp, err := requester.Do(request)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, 200)

	// Set verifyTLS=false, verify an insecure TLS connection is returned even the serverCertificate is available.
	requester, err = defaultRequester(pemCert, false)
	assert.NoError(t, err)
	httpClient, ok = requester.(*http.Client)
	assert.Equal(t, true, ok)
	assert.Equal(t, httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify, !insecure)

	request, err = http.NewRequest("GET", ts.URL, nil)
	assert.NoError(t, err)
	resp, err = requester.Do(request)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, 200)

}

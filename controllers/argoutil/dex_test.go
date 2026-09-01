package argoutil

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	sigyaml "sigs.k8s.io/yaml"
)

const dexConfigTemplate = `
connectors:
- config:
    clientID: system:serviceaccount:test:test-argocd-dex-server
    clientSecret: abcd.efgh.ijkl-mnop-qrst
    groups: []
    insecureCA: true
    issuer: https://kubernetes.default.svc
    redirectURI: https://argocd-server-argocd.apps.example.com/api/dex/callback
  id: openshift
  name: OpenShift
  type: openshift
grpc:
  addr: 0.0.0.0:5557
issuer: https://argocd-server-argocd.apps.example.com/api/dex/callback
logger:
  format: json
  level: INFO
oauth2:
  skipApprovalScreen: true
staticClients:
- id: argo-cd
  name: Argo CD
  redirectURIs:
  - https://argocd-server-argocd.apps.example.com/auth/callback
  secret: abcdefgh
- id: argo-cd-cli
  name: Argo CD CLI
  public: true
  redirectURIs:
  - http://localhost
  - http://localhost:8085/auth/callback
- id: argo-cd-pkce
  name: Argo CD PKCE
  public: true
  redirectURIs:
  - http://localhost:4000/auth/callback
storage:
  type: {{ .StorageType }}
  config:
    {{ .StorageConfig}}
telemetry:
  http: 0.0.0.0:5558
web:
  https: 0.0.0.0:5556
  tlsCert: /tmp/tls.crt
  tlsKey: /tmp/tls.key
`

type StorageTestCase struct {
	Name          string
	StorageType   string
	StorageConfig string
}

func Test_custom_startup_script_dex(t *testing.T) {
	testCases := []StorageTestCase{
		{
			Name:          "Memory Storage",
			StorageType:   "memory",
			StorageConfig: "",
		},
		{
			Name:          "SQLite3 Storage",
			StorageType:   "sqlite3",
			StorageConfig: "file: /tmp/dex.db",
		},
		{
			Name:        "Postgres Storage (Multi-line)",
			StorageType: "postgres",
			StorageConfig: `host: 127.0.0.1
    port: 5432
    user: dex
    password: password
    db: dex`,
		},
		{
			Name:        "Etcd v3",
			StorageType: "etcd",
			StorageConfig: `endpoints:
      - http://localhost:2379
    namespace: my-etcd-namespace/`,
		},
	}

	awkScript := initAwkScript()
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			inputYAML, err := renderConfig(tc)
			require.NoError(t, err)

			outputYAML, err := runAwkTransform(inputYAML, awkScript)
			require.NoError(t, err)

			require.NotContains(t, outputYAML, "type: memory", "found unexpected memory storage type")
			require.Contains(t, outputYAML, "type: kubernetes", "expected kubernetes storage type not found")
			require.Contains(t, outputYAML, "inCluster: true", "expected postgres storage config in-cluster not found")
			require.Contains(t, outputYAML, "telemetry:\n  http: 0.0.0.0:5558", "Adjacent section 'telemetry' was corrupted or lost")

			expectedOutput, err := renderConfig(StorageTestCase{
				Name:          "kubernetes dex storage",
				StorageType:   "kubernetes",
				StorageConfig: "inCluster: true",
			})
			require.NoError(t, err)
			require.EqualValues(t, expectedOutput, outputYAML)
		})
	}
}

// initAwkScript gets the awk script arguments from startup script
func initAwkScript() string {
	for line := range strings.SplitSeq(DexServerCustomStartupScript()[0], "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "awk") {
			script := strings.ReplaceAll(trimmed, "awk ", "")
			quoted := strings.Split(script, "'")
			return quoted[1]
		}
	}
	return ""
}

// renderConfig renders the template given a test case
func renderConfig(tc StorageTestCase) (string, error) {
	tmpl, err := template.New("dexConfig").Parse(dexConfigTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tc); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// runAwkTransform executes the awk script against an input YAML string
func runAwkTransform(inputYAML, awkScript string) (string, error) {
	cmd := exec.Command("awk", awkScript)
	cmd.Stdin = strings.NewReader(inputYAML)

	var outBuffer bytes.Buffer
	var errBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return outBuffer.String(), nil
}

func newFakeClientForCRDs() client.Client {
	sch := runtime.NewScheme()
	apiextensionsv1.AddToScheme(sch)
	return fake.NewClientBuilder().WithScheme(sch).Build()
}

func TestEnsureDexCRDs_CreatesAllCRDs(t *testing.T) {
	cl := newFakeClientForCRDs()
	ctx := context.Background()

	err := EnsureDexCRDs(ctx, cl)
	require.NoError(t, err)

	for _, crd := range DexCRDs {
		obj := &apiextensionsv1.CustomResourceDefinition{}
		key := client.ObjectKey{Name: crd.Plural + ".dex.coreos.com"}
		err := cl.Get(ctx, key, obj)
		assert.NoError(t, err, "CRD %s should exist", crd.Plural)
		assert.Equal(t, "dex.coreos.com", obj.Spec.Group)
		assert.Equal(t, crd.Kind, obj.Spec.Names.Kind)
		assert.Equal(t, crd.Kind+"List", obj.Spec.Names.ListKind)
		assert.Equal(t, crd.Plural, obj.Spec.Names.Plural)
	}
}

func TestEnsureDexCRDs_IsIdempotent(t *testing.T) {
	cl := newFakeClientForCRDs()
	ctx := context.Background()

	require.NoError(t, EnsureDexCRDs(ctx, cl))
	require.NoError(t, EnsureDexCRDs(ctx, cl))

	for _, crd := range DexCRDs {
		obj := &apiextensionsv1.CustomResourceDefinition{}
		key := client.ObjectKey{Name: crd.Plural + ".dex.coreos.com"}
		assert.NoError(t, cl.Get(ctx, key, obj), "CRD %s should still exist after second call", crd.Plural)
	}
}

func TestEnsureDexCRDs_GetError(t *testing.T) {
	sch := runtime.NewScheme()
	apiextensionsv1.AddToScheme(sch)

	wantErr := fmt.Errorf("simulated get error")
	cl := fake.NewClientBuilder().WithScheme(sch).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			return wantErr
		},
	}).Build()

	err := EnsureDexCRDs(context.Background(), cl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch CRD")
}

func TestEnsureDexCRDs_CreateError(t *testing.T) {
	sch := runtime.NewScheme()
	apiextensionsv1.AddToScheme(sch)

	wantErr := fmt.Errorf("simulated create error")
	cl := fake.NewClientBuilder().WithScheme(sch).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			return wantErr
		},
	}).Build()

	err := EnsureDexCRDs(context.Background(), cl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create CRD")
}

func TestEnsureDexCRDs_TemplateProducesValidGVK(t *testing.T) {
	tmpl, err := template.New("crd").Parse(dexCRDTemplate)
	require.NoError(t, err)

	for _, crd := range DexCRDs {
		t.Run(crd.Kind, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, tmpl.Execute(&buf, crd))

			obj := &unstructured.Unstructured{}
			require.NoError(t, sigyaml.Unmarshal(buf.Bytes(), &obj.Object))

			gvk := obj.GroupVersionKind()
			assert.Equal(t, "apiextensions.k8s.io", gvk.Group)
			assert.Equal(t, "v1", gvk.Version)
			assert.Equal(t, "CustomResourceDefinition", gvk.Kind)
			assert.Equal(t, crd.Plural+".dex.coreos.com", obj.GetName())
		})
	}
}

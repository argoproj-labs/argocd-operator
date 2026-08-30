package argoutil

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
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

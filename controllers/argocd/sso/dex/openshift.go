package dex

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

const (
	clientSecretKey = "oidc.dex.clientSecret"
)

func (dr *DexReconciler) getOpenShiftConfig() (string, error) {
	groups := []string{}

	// Allow override of groups from dr.Instance
	if dr.Instance.Spec.SSO.Dex != nil && dr.Instance.Spec.SSO.Dex.Groups != nil {
		groups = dr.Instance.Spec.SSO.Dex.Groups
	}

	connector := DexConnector{
		Type: "openshift",
		ID:   "openshift",
		Name: "OpenShift",
		Config: map[string]interface{}{
			"issuer":       "https://kubernetes.default.svc", // TODO: Should this be hard-coded?
			"clientID":     dr.getOAuthClientID(),
			"clientSecret": clientSecretKey,
			"redirectURI":  dr.GetOAuthRedirectURI(),
			"insecureCA":   true, // TODO: Configure for openshift CA,
			"groups":       groups,
		},
	}

	connectors := make([]DexConnector, 0)
	connectors = append(connectors, connector)
	dex := make(map[string]interface{})
	dex["connectors"] = connectors

	// add dex config from the Argo CD CR
	dexCfgStr := dr.getConfig()
	if err := getMapFromConfigStr(dexCfgStr, dex); err != nil {
		return "", errors.Wrap(err, "getOpenShiftConfig: failed to unmarshal dex configuration")
	}

	bytes, err := yaml.Marshal(dex)
	return string(bytes), err
}

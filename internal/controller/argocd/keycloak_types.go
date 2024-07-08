package argocd

type KeycloakAPIClient struct {
	// Client ID.
	// +kubebuilder:validation:Required
	ClientID string `json:"clientId"`
	// Client name.
	// +optional
	Name string `json:"name,omitempty"`
	// What Client authentication type to use.
	// +optional
	ClientAuthenticatorType string `json:"clientAuthenticatorType,omitempty"`
	// Client Secret. The Operator will automatically create a Secret based on this value.
	// +optional
	Secret string `json:"secret,omitempty"`
	// Application base URL.
	// +optional
	BaseURL string `json:"baseUrl,omitempty"`
	// Application Admin URL.
	// +optional
	AdminURL string `json:"adminUrl,omitempty"`
	// Application root URL.
	// +optional
	RootURL string `json:"rootUrl,omitempty"`
	// A list of valid Redirection URLs.
	// +optional
	RedirectUris []string `json:"redirectUris,omitempty"`
	// A list of valid Web Origins.
	// +optional
	WebOrigins []string `json:"webOrigins,omitempty"`
	// True if Standard flow is enabled.
	// +optional
	StandardFlowEnabled bool `json:"standardFlowEnabled"`
	// A list of default client scopes. Default client scopes are
	// always applied when issuing OpenID Connect tokens or SAML
	// assertions for this client.
	// +optional
	DefaultClientScopes []string `json:"defaultClientScopes,omitempty"`
}

type KeycloakClientScope struct {
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`
	// +optional
	ID string `json:"id,omitempty"`
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	Protocol string `json:"protocol,omitempty"`
	// Protocol Mappers.
	// +optional
	ProtocolMappers []KeycloakProtocolMapper `json:"protocolMappers,omitempty"`
}

type KeycloakProtocolMapper struct {
	// Protocol Mapper ID.
	// +optional
	ID string `json:"id,omitempty"`
	// Protocol Mapper Name.
	// +optional
	Name string `json:"name,omitempty"`
	// Protocol to use.
	// +optional
	Protocol string `json:"protocol,omitempty"`
	// Protocol Mapper to use
	// +optional
	ProtocolMapper string `json:"protocolMapper,omitempty"`
	// Config options.
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type KeycloakIdentityProvider struct {
	// Identity Provider Alias.
	// +optional
	Alias string `json:"alias,omitempty"`
	// Identity Provider Display Name.
	// +optional
	DisplayName string `json:"displayName,omitempty"`
	// Identity Provider ID.
	// +optional
	ProviderID string `json:"providerId,omitempty"`
	// Identity Provider config.
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type TokenResponse struct {
	// Token Response Access Token.
	// +optional
	AccessToken string `json:"access_token"`
	// Token Response Error.
	// +optional
	Error string `json:"error"`
}

// KeycloakPostData defines the values required to update Keycloak Realm.
type keycloakConfig struct {
	ArgoName           string
	ArgoNamespace      string
	Username           string
	Password           string
	KeycloakURL        string
	ArgoCDURL          string
	KeycloakServerCert []byte
	VerifyTLS          bool
}

type oidcConfig struct {
	Name           string   `json:"name"`
	Issuer         string   `json:"issuer"`
	ClientID       string   `json:"clientID"`
	ClientSecret   string   `json:"clientSecret"`
	RequestedScope []string `json:"requestedScopes"`
	RootCA         string   `json:"rootCA,omitempty"`
}

// KeycloakIdentityProviderMapper defines IdentityProvider Mappers
// issue: https://github.com/keycloak/keycloak-operator/issues/471
type KeycloakIdentityProviderMapper struct {
	// Name
	// +optional
	Name string `json:"name,omitempty"`
	// Identity Provider Alias.
	// +optional
	IdentityProviderAlias string `json:"identityProviderAlias,omitempty"`
	// Identity Provider Mapper.
	// +optional
	IdentityProviderMapper string `json:"identityProviderMapper,omitempty"`
	// Identity Provider Mapper config.
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// CustomKeycloakAPIRealm is an extention type of KeycloakAPIRealm as is it does not
// support IdentityProvider Mappers
// issue: https://github.com/keycloak/keycloak-operator/issues/471
type CustomKeycloakAPIRealm struct {
	// Realm name.
	Realm string `json:"realm"`
	// Realm enabled flag.
	// +optional
	Enabled bool `json:"enabled"`
	// Require SSL
	// +optional
	SslRequired string `json:"sslRequired,omitempty"`
	// A set of Keycloak Clients.
	// +optional
	Clients []*KeycloakAPIClient `json:"clients,omitempty"`
	// Client scopes
	// +optional
	ClientScopes []KeycloakClientScope `json:"clientScopes,omitempty"`
	// A set of Identity Providers.
	// +optional
	IdentityProviders []*KeycloakIdentityProvider `json:"identityProviders,omitempty"`
	// KeycloakIdentityProviderMapper defines IdentityProvider Mappers
	// issue: https://github.com/keycloak/keycloak-operator/issues/471
	IdentityProviderMappers []*KeycloakIdentityProviderMapper `json:"identityProviderMappers,omitempty"`
}

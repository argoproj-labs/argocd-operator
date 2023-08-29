package argoutil

import (
	"fmt"

	oappsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	templatev1client "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// GetK8sClient returns a k8s client instance
func GetK8sClient() (*kubernetes.Clientset, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("GetK8sClient: unable to get config: %w", err)
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("GetK8sClient: unable to create client: %w", err)
	}

	return k8sClient, nil
}

// GetOAppsClient returns an openshift apps client instance
func GetOAppsClient() (*oappsv1client.AppsV1Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("GetOAppsClient: unable to get config: %w", err)
	}

	// Initialize deployment config client.
	dcClient, err := oappsv1client.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("GetOAppsClient: unable to create client: %w", err)
	}

	return dcClient, nil
}

// GetConfigClient returns an openshift config client instance
func GetConfigClient() (*configv1client.ConfigV1Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("GetConfigClient: unable to get config: %w", err)
	}

	configClient, err := configv1client.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("GetConfigClient: unable to create client: %w", err)
	}

	return configClient, nil
}

// GetTemplateClient returns a template client instance
func GetTemplateClient() (*templatev1client.TemplateV1Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("GetTemplateClient: unable to get config: %w", err)
	}

	tempalteClient, err := templatev1client.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("GetTemplateClient: unable to create client: %w", err)
	}

	return tempalteClient, nil
}

// GetOAuthClient returns an oauth client instance
func GetOAuthClient() (*oauthclient.OauthV1Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("GetTemplateClient: unable to get config: %w", err)
	}

	oauthClient, err := oauthclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("GetTemplateClient: unable to create client: %w", err)
	}

	return oauthClient, nil
}

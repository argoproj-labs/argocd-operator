package argocdcommon

import (
	"fmt"
	"strings"

	"github.com/argoproj-labs/argocd-operator/common"
)

const (
	maxLabelLength    = 63
	maxHostnameLength = 253
	minFirstLabelSize = 20
)

func GetIngressNginxAnnotations() map[string]string {
	return map[string]string{
		common.NginxIngressK8sKeyForceSSLRedirect: "true",
		common.NginxIngressK8sKeyBackendProtocol:  "true",
	}
}

func GetGRPCIngressNginxAnnotations() map[string]string {
	return map[string]string{
		common.NginxIngressK8sKeyBackendProtocol: "GRPC",
	}
}

// The algorithm used by this function is:
// - If the FIRST label ("console-openshift-console" in the above case) is longer than 63 characters, shorten (truncate the end) it to 63.
// - If any other label is longer than 63 characters, return an error
// - After all the labels are 63 characters or less, check the length of the overall hostname:
//   - If the overall hostname is > 253, then shorten the FIRST label until the host name is < 253
//   - After the FIRST label has been shortened, if it is < 20, then return an error (this is a sanity test to ensure the label is likely to be unique)
func ShortenHostname(hostname string) (string, error) {
	if hostname == "" {
		return "", nil
	}

	// Return the hostname as it is if hostname is already within the size limit
	if len(hostname) <= maxHostnameLength {
		return hostname, nil
	}

	// Split the hostname into labels
	labels := strings.Split(hostname, ".")

	// Check and truncate the FIRST label if longer than 63 characters
	if len(labels[0]) > maxLabelLength {
		labels[0] = labels[0][:maxLabelLength]
	}

	// Check other labels and return an error if any is longer than 63 characters
	for _, label := range labels[1:] {
		if len(label) > maxLabelLength {
			return "", fmt.Errorf("label length exceeds 63 characters")
		}
	}

	// Join the labels back into a hostname
	resultHostname := strings.Join(labels, ".")

	// Check and shorten the overall hostname
	if len(resultHostname) > maxHostnameLength {
		// Shorten the first label until the length is less than 253
		for len(resultHostname) > maxHostnameLength && len(labels[0]) > 20 {
			labels[0] = labels[0][:len(labels[0])-1]
			resultHostname = strings.Join(labels, ".")
		}

		// Check if the first label is still less than 20 characters
		if len(labels[0]) < minFirstLabelSize {
			return "", fmt.Errorf("shortened first label is less than 20 characters")
		}
	}
	return resultHostname, nil
}

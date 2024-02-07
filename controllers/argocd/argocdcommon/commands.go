package argocdcommon

const (
	ArgoCDBinPath    = "/usr/local/bin/argocd"
	CmpServerBinPath = "/var/run/argocd/argocd-cmp-server"
)

// getArgoCmpServerInitCommand will return the command for the ArgoCD CMP Server init container
func GetArgoCmpServerInitCommand() []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "cp")
	cmd = append(cmd, "-n")
	cmd = append(cmd, ArgoCDBinPath)
	cmd = append(cmd, CmpServerBinPath)
	return cmd
}

# API Documentation
<p>Packages:</p>
<ul>
   <li>
      <a href="#argoproj.io%2fv1alpha1">argoproj.io/v1alpha1</a>
   </li>
</ul>
<h2 id="argoproj.io/v1alpha1">argoproj.io/v1alpha1</h2>
<p>
<p>Package v1alpha1 contains API Schema definitions for the argoproj v1alpha1 API group</p>
</p>
Resource Types:
<ul></ul>
<h3 id="argoproj.io/v1alpha1.ArgoCD">ArgoCD</h3>
<p>
<p>ArgoCD is the Schema for the argocds API</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>metadata</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#objectmeta-v1-meta">
            Kubernetes meta/v1.ObjectMeta
            </a>
            </em>
         </td>
         <td>
            Refer to the Kubernetes API documentation for the fields of the
            <code>metadata</code> field.
         </td>
      </tr>
      <tr>
         <td>
            <code>spec</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDSpec">
            ArgoCDSpec
            </a>
            </em>
         </td>
         <td>
            <br/>
            <br/>
            <table>
               <tr>
                  <td>
                     <code>applicationInstanceLabelKey</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>ApplicationInstanceLabelKey is the key name where Argo CD injects the app name as a tracking label.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>configManagementPlugins</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>ConfigManagementPlugins is used to specify additional config management plugins.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>controller</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDApplicationControllerSpec">
                     ArgoCDApplicationControllerSpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>Controller defines the Application Controller options for ArgoCD.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>gaTrackingID</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>GATrackingID is the google analytics tracking ID to use.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>gaAnonymizeUsers</code></br>
                     <em>
                     bool
                     </em>
                  </td>
                  <td>
                     <p>GAAnonymizeUsers toggles user IDs being hashed before sending to google analytics.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>ha</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDHASpec">
                     ArgoCDHASpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>HA options for High Availability support for the Redis component.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>helpChatURL</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>HelpChatURL is the URL for getting chat help, this will typically be your Slack channel for support.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>helpChatText</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>HelpChatText is the text for getting chat help, defaults to &ldquo;Chat now!&rdquo;</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>image</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>Image is the ArgoCD container image for all ArgoCD components.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>import</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDImportSpec">
                     ArgoCDImportSpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>Import is the import/restore options for ArgoCD.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>initialRepositories</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>InitialRepositories to configure Argo CD with upon creation of the cluster.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>initialSSHKnownHosts</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>InitialSSHKnownHosts defines the SSH known hosts data upon creation of the cluster for connecting Git repositories via SSH.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>kustomizeBuildOptions</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>KustomizeBuildOptions is used to specify build options/parameters to use with <code>kustomize build</code>.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>notifications</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDNotificationsSpec">
                     ArgoCDNotificationsSpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>Notifications controls the desired state of Argo CD Notifications controller.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>oidcConfig</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>OIDCConfig is the OIDC configuration as an alternative to dex.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>prometheus</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDPrometheusSpec">
                     ArgoCDPrometheusSpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>Prometheus defines the Prometheus server options for ArgoCD.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>rbac</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDRBACSpec">
                     ArgoCDRBACSpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>RBAC defines the RBAC configuration for Argo CD.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>redis</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDRedisSpec">
                     ArgoCDRedisSpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>Redis defines the Redis server options for ArgoCD.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>repo</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDRepoSpec">
                     ArgoCDRepoSpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>Repo defines the repo server options for Argo CD.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>repositoryCredentials</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>RepositoryCredentials are the Git pull credentials to configure Argo CD with upon creation of the cluster.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>resourceCustomizations</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>ResourceCustomizations customizes resource behavior. Keys are in the form: group/Kind.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>resourceExclusions</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>ResourceExclusions is used to completely ignore entire classes of resource group/kinds.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>server</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDServerSpec">
                     ArgoCDServerSpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>Server defines the options for the ArgoCD Server component.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>sso</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDSSOSpec">
                     ArgoCDSSOSpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>SSO defines the Single Sign-on configuration for Argo CD.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>statusBadgeEnabled</code></br>
                     <em>
                     bool
                     </em>
                  </td>
                  <td>
                     <p>StatusBadgeEnabled toggles application status badge feature.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>tls</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDTLSSpec">
                     ArgoCDTLSSpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>TLS defines the TLS options for ArgoCD.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>usersAnonymousEnabled</code></br>
                     <em>
                     bool
                     </em>
                  </td>
                  <td>
                     <p>UsersAnonymousEnabled toggles anonymous user access.
                        The anonymous users get default role permissions specified argocd-rbac-cm.
                     </p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>version</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>Version is the tag to use with the ArgoCD container image for all ArgoCD components.</p>
                  </td>
               </tr>
            </table>
         </td>
      </tr>
      <tr>
         <td>
            <code>status</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDStatus">
            ArgoCDStatus
            </a>
            </em>
         </td>
         <td></td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDApplicationControllerProcessorsSpec">ArgoCDApplicationControllerProcessorsSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDApplicationControllerSpec">ArgoCDApplicationControllerSpec</a>)
</p>
<p>
<p>ArgoCDApplicationControllerProcessorsSpec defines the options for the ArgoCD Application Controller processors.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>operation</code></br>
            <em>
            int32
            </em>
         </td>
         <td>
            <p>Operation is the number of application operation processors.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>status</code></br>
            <em>
            int32
            </em>
         </td>
         <td>
            <p>Status is the number of application status processors.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDApplicationControllerSpec">ArgoCDApplicationControllerSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDApplicationControllerSpec defines the options for the ArgoCD Application Controller component.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>processors</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDApplicationControllerProcessorsSpec">
            ArgoCDApplicationControllerProcessorsSpec
            </a>
            </em>
         </td>
         <td>
            <p>Processors contains the options for the Application Controller processors.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>resources</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
            Kubernetes core/v1.ResourceRequirements
            </a>
            </em>
         </td>
         <td>
            <p>Resources defines the Compute Resources required by the container for the Application Controller.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDCASpec">ArgoCDCASpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDTLSSpec">ArgoCDTLSSpec</a>)
</p>
<p>
<p>ArgoCDCASpec defines the CA options for ArgCD.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>configMapName</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>ConfigMapName is the name of the ConfigMap containing the CA Certificate.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>secretName</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>SecretName is the name of the Secret containing the CA Certificate and Key.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDCertificateSpec">ArgoCDCertificateSpec</h3>
<p>
<p>ArgoCDCertificateSpec defines the options for the ArgoCD certificates.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>secretName</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>SecretName is the name of the Secret containing the Certificate and Key.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDDexOAuthSpec">ArgoCDDexOAuthSpec</h3>
<p>
<p>ArgoCDDexOAuthSpec defines the desired state for the Dex OAuth configuration.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>enabled</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>Enabled will toggle OAuth support for the Dex server.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDDexSpec">ArgoCDDexSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDDexSpec defines the desired state for the Dex server component.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>config</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Config is the dex connector configuration.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>image</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Image is the Dex container image.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>openShiftOAuth</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>OpenShiftOAuth enables OpenShift OAuth authentication for the Dex server.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>resources</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
            Kubernetes core/v1.ResourceRequirements
            </a>
            </em>
         </td>
         <td>
            <p>Resources defines the Compute Resources required by the container for Dex.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>version</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Version is the Dex container image tag.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDExport">ArgoCDExport</h3>
<p>
<p>ArgoCDExport is the Schema for the argocdexports API</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>metadata</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
            Kubernetes meta/v1.ObjectMeta
            </a>
            </em>
         </td>
         <td>
            Refer to the Kubernetes API documentation for the fields of the
            <code>metadata</code> field.
         </td>
      </tr>
      <tr>
         <td>
            <code>spec</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDExportSpec">
            ArgoCDExportSpec
            </a>
            </em>
         </td>
         <td>
            <br/>
            <br/>
            <table>
               <tr>
                  <td>
                     <code>argocd</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>Argocd is the name of the ArgoCD instance to export.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>image</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>Image is the container image to use for the export Job.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>schedule</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>Schedule in Cron format, see <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a>.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>storage</code></br>
                     <em>
                     <a href="#argoproj.io/v1alpha1.ArgoCDExportStorageSpec">
                     ArgoCDExportStorageSpec
                     </a>
                     </em>
                  </td>
                  <td>
                     <p>Storage defines the storage configuration options.</p>
                  </td>
               </tr>
               <tr>
                  <td>
                     <code>version</code></br>
                     <em>
                     string
                     </em>
                  </td>
                  <td>
                     <p>Version is the tag/digest to use for the export Job container image.</p>
                  </td>
               </tr>
            </table>
         </td>
      </tr>
      <tr>
         <td>
            <code>status</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDExportStatus">
            ArgoCDExportStatus
            </a>
            </em>
         </td>
         <td></td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDExportSpec">ArgoCDExportSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDExport">ArgoCDExport</a>)
</p>
<p>
<p>ArgoCDExportSpec defines the desired state of ArgoCDExport</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>argocd</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Argocd is the name of the ArgoCD instance to export.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>image</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Image is the container image to use for the export Job.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>schedule</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Schedule in Cron format, see <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a>.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>storage</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDExportStorageSpec">
            ArgoCDExportStorageSpec
            </a>
            </em>
         </td>
         <td>
            <p>Storage defines the storage configuration options.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>version</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Version is the tag/digest to use for the export Job container image.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDExportStatus">ArgoCDExportStatus</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDExport">ArgoCDExport</a>)
</p>
<p>
<p>ArgoCDExportStatus defines the observed state of ArgoCDExport</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>phase</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Phase is a simple, high-level summary of where the ArgoCDExport is in its lifecycle.
               There are five possible phase values:
               Pending: The ArgoCDExport has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
               Running: All of the containers for the ArgoCDExport are still running, or in the process of starting or restarting.
               Succeeded: All containers for the ArgoCDExport have terminated in success, and will not be restarted.
               Failed: At least one container has terminated in failure, either exited with non-zero status or was terminated by the system.
               Unknown: For some reason the state of the ArgoCDExport could not be obtained.
            </p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDExportStorageSpec">ArgoCDExportStorageSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDExportSpec">ArgoCDExportSpec</a>)
</p>
<p>
<p>ArgoCDExportStorageSpec defines the desired state for ArgoCDExport storage options.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>backend</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Backend defines the storage backend to use, must be &ldquo;local&rdquo; (the default), &ldquo;aws&rdquo;, &ldquo;azure&rdquo; or &ldquo;gcp&rdquo;.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>pvc</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#persistentvolumeclaimspec-v1-core">
            Kubernetes core/v1.PersistentVolumeClaimSpec
            </a>
            </em>
         </td>
         <td>
            <p>PVC is the desired characteristics for a PersistentVolumeClaim.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>secretName</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>SecretName is the name of a Secret with encryption key, credentials, etc.</p>
         </td>
      </tr>
   </tbody>
</table>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>enabled</code></br>
            <em>
            bool
            </em>
         </td>
      </tr>
      <tr>
         <td>
            <code>host</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Host is the hostname to use for Ingress/Route resources.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>image</code></br>
            <em>
            string
            </em>
         </td>
      </tr>
      <tr>
         <td>
            <code>ingress</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDIngressSpec">
            ArgoCDIngressSpec
            </a>
            </em>
         </td>
      </tr>
      <tr>
         <td>
            <code>resources</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
            Kubernetes core/v1.ResourceRequirements
            </a>
            </em>
         </td>
      </tr>
      <tr>
         <td>
            <code>route</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDRouteSpec">
            ArgoCDRouteSpec
            </a>
            </em>
         </td>
      </tr>
      <tr>
         <td>
            <code>size</code></br>
            <em>
            int32
            </em>
         </td>
      </tr>
      <tr>
         <td>
            <code>version</code></br>
            <em>
            string
            </em>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDHASpec">ArgoCDHASpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDHASpec defines the desired state for High Availability support for Argo CD.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>enabled</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>Enabled will toggle HA support globally for Argo CD.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>redisProxyImage</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>The Redis HAProxy container image. This overrides the "ARGOCD_REDIS_HA_PROXY_IMAGE" environment variable.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>redisProxyVersion</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>The tag to use for the Redis HAProxy container image.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>resources</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
            Kubernetes core/v1.ResourceRequirements
            </a>
            </em>
         </td>
         <td>
            <p>Resources defines the Compute Resources required by the container for Redis HA.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDImportSpec">ArgoCDImportSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDImportSpec defines the desired state for the ArgoCD import/restore process.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>name</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Name of an ArgoCDExport from which to import data.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>namespace</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Namespace for the ArgoCDExport, defaults to the same namespace as the ArgoCD.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDIngressSpec">ArgoCDIngressSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDPrometheusSpec">ArgoCDPrometheusSpec</a>, 
   <a href="#argoproj.io/v1alpha1.ArgoCDServerGRPCSpec">ArgoCDServerGRPCSpec</a>, 
   <a href="#argoproj.io/v1alpha1.ArgoCDServerSpec">ArgoCDServerSpec</a>)
</p>
<p>
<p>ArgoCDIngressSpec defines the desired state for the Ingress resources.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>annotations</code></br>
            <em>
            map[string]string
            </em>
         </td>
         <td>
            <p>Annotations is the map of annotations to apply to the Ingress.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>enabled</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>Enabled will toggle the creation of the Ingress.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>path</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Path used for the Ingress resource.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>tls</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#ingresstls-v1-networking-k8s-io">
            []Kubernetes networking.k8s.io/v1.IngressTLS
            </a>
            </em>
         </td>
         <td>
            <em>(Optional)</em>
            <p>TLS configuration. Currently the Ingress only supports a single TLS
               port, 443. If multiple members of this list specify different hosts, they
               will be multiplexed on the same port according to the hostname specified
               through the SNI TLS extension, if the ingress controller fulfilling the
               ingress supports SNI.
            </p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDNotificationsSpec">ArgoCDNotificationsSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDNotificationsSpec defines the desired state for the Notifications controller component.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>enabled</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>Enabled is the toggle that determines whether notifications controller should be started or not. </p>
         </td>
      </tr>
      <tr>
         <td>
            <code>image</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Image is the Argo CD container image.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>replicas</code></br>
            <em>
            int32
            </em>
         </td>
         <td>
            <p>Replicas determins the number of replicas for notifications controller.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>resources</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
            Kubernetes core/v1.ResourceRequirements
            </a>
            </em>
         </td>
         <td>
            <p>Resources defines the Compute Resources required by the container for Notifications controller.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>version</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Version is the Argo CD container image tag.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDPrometheusSpec">ArgoCDPrometheusSpec</h3><h3 id="argoproj.io/v1alpha1.ArgoCDPrometheusSpec">ArgoCDPrometheusSpec
</h3>
<h3 id="argoproj.io/v1alpha1.ArgoCDKeycloakSpec">ArgoCDKeycloakSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSSOSpec">ArgoCDSSOSpec</a>)
</p>
<p>
<p>ArgoCDKeycloakSpec Keycloak contains the configuration for Argo CD keycloak authentication (previously found under cr.spec.sso)</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>image</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Image is the Keycloak container image.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>resources</code></br>
            <em>
               <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#resourcerequirements-v1-core">
                  Kubernetes core/v1.ResourceRequirements
            </em>
         </td>
         <td>
         <p> Resources defines the Compute Resources required by the container for Keycloak.</p>
         </td>
      </tr>
      <tr>
      <td>
        <code>version</code></br>
      <em>
        string
      </em>
      </td>
      <td>
        <p>Version is the Keycloak container image tag.</p>
      </td>
      </tr>
      <tr>
      <td>
        <code>verifyTLS</code></br>
      <em>
      bool
      </em>
      </td>
      <td>
      <p>VerifyTLS set to false disables strict TLS validation.</p>
      </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDPrometheusSpec">ArgoCDPrometheusSpec</h3>
<p>
(<em>Appears on:</em>
<a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDPrometheusSpec defines the desired state for the Prometheus component.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>enabled</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>Enabled will toggle Prometheus support globally for ArgoCD.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>host</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Host is the hostname to use for Ingress/Route resources.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>ingress</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDIngressSpec">
            ArgoCDIngressSpec
            </a>
            </em>
         </td>
         <td>
            <p>Ingress defines the desired state for an Ingress for the Prometheus component.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>route</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDRouteSpec">
            ArgoCDRouteSpec
            </a>
            </em>
         </td>
         <td>
            <p>Route defines the desired state for an OpenShift Route for the Prometheus component.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>size</code></br>
            <em>
            int32
            </em>
         </td>
         <td>
            <p>Size is the replica count for the Prometheus StatefulSet.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDRBACSpec">ArgoCDRBACSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDRBACSpec defines the desired state for the Argo CD RBAC configuration.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>defaultPolicy</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>DefaultPolicy is the name of the default role which Argo CD will falls back to, when
               authorizing API requests (optional). If omitted or empty, users may be still be able to login,
               but will see no apps, projects, etc&hellip;
            </p>
         </td>
      </tr>
      <tr>
         <td>
            <code>policy</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Policy is CSV containing user-defined RBAC policies and role definitions.
               Policy rules are in the form:
               p, subject, resource, action, object, effect
               Role definitions and bindings are in the form:
               g, subject, inherited-subject
               See <a href="https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/rbac.md">https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/rbac.md</a> for additional information.
            </p>
         </td>
      </tr>
      <tr>
         <td>
            <code>scopes</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Scopes controls which OIDC scopes to examine during rbac enforcement (in addition to <code>sub</code> scope).
               If omitted, defaults to: &lsquo;[groups]&rsquo;.
            </p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDRedisSpec">ArgoCDRedisSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDRedisSpec defines the desired state for the Redis server component.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>image</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Image is the Redis container image.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>resources</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
            Kubernetes core/v1.ResourceRequirements
            </a>
            </em>
         </td>
         <td>
            <p>Resources defines the Compute Resources required by the container for Redis.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>version</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Version is the Redis container image tag.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDRepoSpec">ArgoCDRepoSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDRepoSpec defines the desired state for the Argo CD repo server component.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>mountsatoken</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>MountSAToken describes whether you would like to have the Repo server mount the service account token</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>resources</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
            Kubernetes core/v1.ResourceRequirements
            </a>
            </em>
         </td>
         <td>
            <p>Resources defines the Compute Resources required by the container for Redis.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>serviceaccount</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>ServiceAccount defines the ServiceAccount user that you would like the Repo server to use</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDRouteSpec">ArgoCDRouteSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDPrometheusSpec">ArgoCDPrometheusSpec</a>, 
   <a href="#argoproj.io/v1alpha1.ArgoCDServerSpec">ArgoCDServerSpec</a>)
</p>
<p>
<p>ArgoCDRouteSpec defines the desired state for an OpenShift Route.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>annotations</code></br>
            <em>
            map[string]string
            </em>
         </td>
         <td>
            <p>Annotations is the map of annotations to use for the Route resource.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>enabled</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>Enabled will toggle the creation of the OpenShift Route.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>path</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Path the router watches for, to route traffic for to the service.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>tls</code></br>
            <em>
            github.com/openshift/api/route/v1.TLSConfig
            </em>
         </td>
         <td>
            <p>TLS provides the ability to configure certificates and termination for the Route.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>wildcardPolicy</code></br>
            <em>
            github.com/openshift/api/route/v1.WildcardPolicyType
            </em>
         </td>
         <td>
            <p>WildcardPolicy if any for the route. Currently only &lsquo;Subdomain&rsquo; or &lsquo;None&rsquo; is allowed.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDServerAutoscaleSpec">ArgoCDServerAutoscaleSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDServerSpec">ArgoCDServerSpec</a>)
</p>
<p>
<p>ArgoCDServerAutoscaleSpec defines the desired state for autoscaling the Argo CD Server component.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>enabled</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>Enabled will toggle autoscaling support for the Argo CD Server component.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>hpa</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#horizontalpodautoscalerspec-v1-autoscaling">
            Kubernetes autoscaling/v1.HorizontalPodAutoscalerSpec
            </a>
            </em>
         </td>
         <td>
            <p>HPA defines the HorizontalPodAutoscaler options for the Argo CD Server component.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDServerGRPCSpec">ArgoCDServerGRPCSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDServerSpec">ArgoCDServerSpec</a>)
</p>
<p>
<p>ArgoCDServerGRPCSpec defines the desired state for the Argo CD Server GRPC options.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>host</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Host is the hostname to use for Ingress/Route resources.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>ingress</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDIngressSpec">
            ArgoCDIngressSpec
            </a>
            </em>
         </td>
         <td>
            <p>Ingress defines the desired state for the Argo CD Server GRPC Ingress.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDServerServiceSpec">ArgoCDServerServiceSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDServerSpec">ArgoCDServerSpec</a>)
</p>
<p>
<p>ArgoCDServerServiceSpec defines the Service options for Argo CD Server component.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>type</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#servicetype-v1-core">
            Kubernetes core/v1.ServiceType
            </a>
            </em>
         </td>
         <td>
            <p>Type is the ServiceType to use for the Service resource.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDServerSpec">ArgoCDServerSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDServerSpec defines the options for the ArgoCD Server component.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>autoscale</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDServerAutoscaleSpec">
            ArgoCDServerAutoscaleSpec
            </a>
            </em>
         </td>
         <td>
            <p>Autoscale defines the autoscale options for the Argo CD Server component.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>grpc</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDServerGRPCSpec">
            ArgoCDServerGRPCSpec
            </a>
            </em>
         </td>
         <td>
            <p>GRPC defines the state for the Argo CD Server GRPC options.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>host</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Host is the hostname to use for Ingress/Route resources.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>ingress</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDIngressSpec">
            ArgoCDIngressSpec
            </a>
            </em>
         </td>
         <td>
            <p>Ingress defines the desired state for an Ingress for the Argo CD Server component.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>insecure</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>Insecure toggles the insecure flag.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>resources</code></br>
            <em>
            <a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
            Kubernetes core/v1.ResourceRequirements
            </a>
            </em>
         </td>
         <td>
            <p>Resources defines the Compute Resources required by the container for the Argo CD server component.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>route</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDRouteSpec">
            ArgoCDRouteSpec
            </a>
            </em>
         </td>
         <td>
            <p>Route defines the desired state for an OpenShift Route for the Argo CD Server component.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>service</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDServerServiceSpec">
            ArgoCDServerServiceSpec
            </a>
            </em>
         </td>
         <td>
            <p>Service defines the options for the Service backing the ArgoCD Server component.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCD">ArgoCD</a>)
</p>
<p>
<p>ArgoCDSpec defines the desired state of ArgoCD</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>applicationInstanceLabelKey</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>ApplicationInstanceLabelKey is the key name where Argo CD injects the app name as a tracking label.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>configManagementPlugins</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>ConfigManagementPlugins is used to specify additional config management plugins.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>controller</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDApplicationControllerSpec">
            ArgoCDApplicationControllerSpec
            </a>
            </em>
         </td>
         <td>
            <p>Controller defines the Application Controller options for ArgoCD.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>gaTrackingID</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>GATrackingID is the google analytics tracking ID to use.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>gaAnonymizeUsers</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>GAAnonymizeUsers toggles user IDs being hashed before sending to google analytics.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>ha</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDHASpec">
            ArgoCDHASpec
            </a>
            </em>
         </td>
         <td>
            <p>HA options for High Availability support for the Redis component.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>helpChatURL</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>HelpChatURL is the URL for getting chat help, this will typically be your Slack channel for support.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>helpChatText</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>HelpChatText is the text for getting chat help, defaults to &ldquo;Chat now!&rdquo;</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>image</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Image is the ArgoCD container image for all ArgoCD components.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>import</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDImportSpec">
            ArgoCDImportSpec
            </a>
            </em>
         </td>
         <td>
            <p>Import is the import/restore options for ArgoCD.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>initialRepositories</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>InitialRepositories to configure Argo CD with upon creation of the cluster.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>initialSSHKnownHosts</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>InitialSSHKnownHosts defines the SSH known hosts data upon creation of the cluster for connecting Git repositories via SSH.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>kustomizeBuildOptions</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>KustomizeBuildOptions is used to specify build options/parameters to use with <code>kustomize build</code>.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>oidcConfig</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>OIDCConfig is the OIDC configuration as an alternative to dex.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>prometheus</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDPrometheusSpec">
            ArgoCDPrometheusSpec
            </a>
            </em>
         </td>
         <td>
            <p>Prometheus defines the Prometheus server options for ArgoCD.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>rbac</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDRBACSpec">
            ArgoCDRBACSpec
            </a>
            </em>
         </td>
         <td>
            <p>RBAC defines the RBAC configuration for Argo CD.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>redis</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDRedisSpec">
            ArgoCDRedisSpec
            </a>
            </em>
         </td>
         <td>
            <p>Redis defines the Redis server options for ArgoCD.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>repo</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDRepoSpec">
            ArgoCDRepoSpec
            </a>
            </em>
         </td>
         <td>
            <p>Repo defines the repo server options for Argo CD.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>repositoryCredentials</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>RepositoryCredentials are the Git pull credentials to configure Argo CD with upon creation of the cluster.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>resourceCustomizations</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>ResourceCustomizations customizes resource behavior. Keys are in the form: group/Kind.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>resourceExclusions</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>ResourceExclusions is used to completely ignore entire classes of resource group/kinds.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>server</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDServerSpec">
            ArgoCDServerSpec
            </a>
            </em>
         </td>
         <td>
            <p>Server defines the options for the ArgoCD Server component.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>sso</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDSSOSpec">
            ArgoCDSSOSpec
            </a>
            </em>
         </td>
         <td>
            <p>SSO defines the Single Sign-on configuration for Argo CD.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>statusBadgeEnabled</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>StatusBadgeEnabled toggles application status badge feature.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>tls</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDTLSSpec">
            ArgoCDTLSSpec
            </a>
            </em>
         </td>
         <td>
            <p>TLS defines the TLS options for ArgoCD.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>usersAnonymousEnabled</code></br>
            <em>
            bool
            </em>
         </td>
         <td>
            <p>UsersAnonymousEnabled toggles anonymous user access.
               The anonymous users get default role permissions specified argocd-rbac-cm.
            </p>
         </td>
      </tr>
      <tr>
         <td>
            <code>version</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Version is the tag to use with the ArgoCD container image for all ArgoCD components.</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDSSOSpec">ArgoCDSSOSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDSSOSpec defines the Single Sign-on configuration for Argo CD.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>dex</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDDexSpec">
            ArgoCDDexSpec
            </em>
         </td>
         <td>
            <p>Dex contains the configuration for Argo CD dex authentication (previously found under cr.spec.dex)</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>keycloak</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDKeycloakSpec">
            ArgoCDKeycloakSpec
            </em>
         </td>
         <td>
            <p>Keycloak contains the configuration for Argo CD keycloak authentication (previously found under cr.spec.sso)</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>provider</code></br>
            <em>
            SSOProviderType
            </em>
         </td>
         <td>
            <p>Provider installs and configures the given SSO Provider with Argo CD.
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDStatus">ArgoCDStatus</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCD">ArgoCD</a>)
</p>
<p>
<p>ArgoCDStatus defines the observed state of ArgoCD</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>applicationController</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>ApplicationController is a simple, high-level summary of where the Argo CD application controller component is in its lifecycle.
               There are five possible ApplicationController values:
               Pending: The Argo CD application controller component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
               Running: All of the required Pods for the Argo CD application controller component are in a Ready state.
               Failed: At least one of the  Argo CD application controller component Pods had a failure.
               Unknown: For some reason the state of the Argo CD application controller component could not be obtained.
            </p>
         </td>
      </tr>
      <tr>
         <td>
            <code>notifications</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Notifications is a simple, high-level summary of where the Argo CD Notifications controller component is in its lifecycle.
               There are four possible notifications values:
               Pending: The Argo CD Notifications controller component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
               Running: All of the required Pods for the Argo CD Notifications controller component are in a Ready state.
               Failed: At least one of the  Argo CD Notifications controller component Pods had a failure.
               Unknown: For some reason the state of the Argo CD Notifications controller component could not be obtained.
            </p>
         </td>
      </tr>
      <tr>
         <td>
            <code>phase</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Phase is a simple, high-level summary of where the ArgoCD is in its lifecycle.
               There are five possible phase values:
               Pending: The ArgoCD has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
               Available: All of the resources for the ArgoCD are ready.
               Failed: At least one resource has experienced a failure.
               Unknown: For some reason the state of the ArgoCD phase could not be obtained.
            </p>
         </td>
      </tr>
      <tr>
         <td>
            <code>redis</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Redis is a simple, high-level summary of where the Argo CD Redis component is in its lifecycle.
               There are five possible redis values:
               Pending: The Argo CD Redis component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
               Running: All of the required Pods for the Argo CD Redis component are in a Ready state.
               Failed: At least one of the  Argo CD Redis component Pods had a failure.
               Unknown: For some reason the state of the Argo CD Redis component could not be obtained.
            </p>
         </td>
      </tr>
      <tr>
         <td>
            <code>repo</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Repo is a simple, high-level summary of where the Argo CD Repo component is in its lifecycle.
               There are five possible repo values:
               Pending: The Argo CD Repo component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
               Running: All of the required Pods for the Argo CD Repo component are in a Ready state.
               Failed: At least one of the  Argo CD Repo component Pods had a failure.
               Unknown: For some reason the state of the Argo CD Repo component could not be obtained.
            </p>
         </td>
      </tr>
      <tr>
         <td>
            <code>server</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Server is a simple, high-level summary of where the Argo CD server component is in its lifecycle.
               There are five possible server values:
               Pending: The Argo CD server component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
               Running: All of the required Pods for the Argo CD server component are in a Ready state.
               Failed: At least one of the  Argo CD server component Pods had a failure.
               Unknown: For some reason the state of the Argo CD server component could not be obtained.
            </p>
         </td>
      </tr>
      <tr>
         <td>
            <code>sso</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>SSO is a simple, high-level summary of where the Argo CD SSO(Dex/Keycloak) component is in its lifecycle.
               There are four possible server values:
               Pending: The Argo CD SSO component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
               Running: All of the required Pods for the Argo CD SSO component are in a Ready state.
               Failed: At least one of the  Argo CD SSO component Pods had a failure.
               Unknown: The state of the Argo CD SSO component could not be obtained.
            </p>
         </td>
      </tr>
      <tr>
         <td>
            <code>host</code></br>
            <em>
            string
            </em>
         </td>
         <td>
            <p>Host is the url for the hostname to use for Ingress/Route resources</p>
         </td>
      </tr>
   </tbody>
</table>
<h3 id="argoproj.io/v1alpha1.ArgoCDTLSSpec">ArgoCDTLSSpec</h3>
<p>
   (<em>Appears on:</em>
   <a href="#argoproj.io/v1alpha1.ArgoCDSpec">ArgoCDSpec</a>)
</p>
<p>
<p>ArgoCDTLSSpec defines the TLS options for ArgCD.</p>
</p>
<table>
   <thead>
      <tr>
         <th>Field</th>
         <th>Description</th>
      </tr>
   </thead>
   <tbody>
      <tr>
         <td>
            <code>ca</code></br>
            <em>
            <a href="#argoproj.io/v1alpha1.ArgoCDCASpec">
            ArgoCDCASpec
            </a>
            </em>
         </td>
         <td>
            <p>CA defines the CA options.</p>
         </td>
      </tr>
      <tr>
         <td>
            <code>initialCerts</code></br>
            <em>
            map[string]string
            </em>
         </td>
         <td>
            <p>InitialCerts defines custom TLS certificates upon creation of the cluster for connecting Git repositories via HTTPS.</p>
         </td>
      </tr>
   </tbody>
</table>
<hr/>
<p><em>
   Generated with <code>gen-crd-api-reference-docs</code>
   on git commit <code>c13da2a</code>.
   </em>
</p>
<h2 id="argoproj.io/v1alpha1">argoproj.io/v1alpha1</h2>
<p>
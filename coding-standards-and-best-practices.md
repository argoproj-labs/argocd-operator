# General coding standards and practices

- Only provide input parameters that are required by a function, instead of passing the entire custom resource. This allows for easier testing and trims down functions' access to bare minimums
- Make sure a function is not trying to do more than it is supposed to do - remember the package responsibility and decide if a given function should be performing an action or not
- Add brief comments explaining what a function does at the top of each function (use chatGPT if required)
- Coding style should be uniform across the project - if something similar to what you are writing has been done elsewhere, try to stick to the same code format
- Don't hard code any recurring strings- create a new constant for it in the package's `constants.go`
- If adding new functionality in the `util` package - check if the function would fit in any existing file, if not create a new one. It's better to have many focused files within the package (`strings.go` , `maps.go` , `resource.go` etc) than 1 big file called `util.go` with a random assortment of functions in it


# Controller packages, resources & testing

- Each package must have its own constants.go if we define any package/controller specific constants
- Each package must have its own `helper.go` to contain functions related to the package but not directly to the reconciliation of a particular resource
- Each managed resource's reconcile function should be in a dedicated file for that resource type
- Each resource created by the operator should have the correct set of labels with appropriate values:
	- `app.kubernetes.io/name` - `<resource-name>`
	- `app.kubernetes.io/instance` - `<argocd-instance-name>`
	- `app.kubernetes.io/part-of` - `argocd`
	- `app.kubernetes.io/component` - `<component-name>`
	- `app.kubernetes.io/managed-by` - `argocd-operator`
- Each package should define common testing variables/functions in the `<package_name>_test.go` file, which should be accessed across the package
- Each controller package will define a number of helper functions in addition to the main resource reconciliation functions. Such functions must either access common variables through that controller's reconciler (as a receiver) or as function parameters. When deciding whether a given function should use a receiver vs parameters, consider the following:
	- If a function is using a receiver, it is a function that is internal to that controller package, and defines a behavior of that controller. It cannot be accessed/invoked by any function in a different controller package without an instance of that controller's reconciler. Typically functions like `getControllerResourceRequirements` or `getControllerImage` are specific to a controller and don't need to be accessed from outside, therefore they should use receivers, essentially making them private functions (only accessible within the package).
	- If a function needs to be accessed from outside the package (i.e by other controllers etc) then it should accept parameters, and not specify a receiver. This allows anyone with the correct parameters to access this function without needing an instance of that controller's reconciler. Functions like `GetClusterSecrets` in the secret controller, are good candidates for such public facing functions and should accept parameters.


# Helper functions

- `controllers/argocd/argocdcommon` is a package containing generic functions that need to be accessed across all controllers. These functions are NOT utility functions that belong to either `pkg/util` or `pkg/argoutil`
packages. These functions form a core part of the operator's logic. They have only been isolated to this package so they can be written once and accessed by whoever needs to. Examples of such functions are:
	- `UpdateIfChanged` to check and update any resources that have drifted
	- `GetContainerImage` to get container images for any component based on first checking the CR, then env vars and then resorting to defaults 
	- `TriggerRollout` for deployments and statefulsets etc

Use this package to store core logic function that need to be accessed by all components 

# Utility functions

- Utility functions are generic go functions that perform low level tasks that are not project specific. These functions likely deal with go data structures or perform generic functions that can be used in any go project.
- Utility functions should not be aware of kubernetes or Argo CD in anyway 
- Some common examples are string manipulation, merging 2 maps etc.
- All utility functions should not be dumped into a single `util.go` file. Each utility function will have a specific domain that it belongs to, and utility functions of the same domain should be grouped together into a single file
that is representative of this domain, and placed in the existing `util` package. Examples of this are:
	- `string.go` containing string manipulation functions
	- `client.go` containing client generation functions
	- `map.go` containing map manipulation functions
- When adding a new function to `util` package, check if this function fits into any of the existing files. If not, create a new file that will contain this and similar functions moving forward. The goal is to have a flat file structure that has files that group similar utility functions together by domain
- Main rule of thumb to keep in mind when deciding if a given function should be considered a 'utility' function or not is - Does this function care about Argo CD or Argo CD Operator in any way or not. If the answer is yes, it should likely not be in the utility package, as it is operator specific.
- If there are helper functions that are kubernetes/argo-cd/operator specific, put them under the `pkg/argoutil` package. Examples of what should be in this package are:
  - `client.go` - A file to compose clients to talk to different API groups
  - `log.go` - For Argo CD specific logging levels
  - `auth.go` - For Argo CD authentication helper functions

# Constants

All existing constants can be broken down into the following categories and sub-categories:
- defaults: constants that define fall back values and only get activated if user has not explicitly set a value for a given field 
  - No specific sub categories; defaults can be grouped by function - image/resource constraints etc
- env var: constants that store environment variable values
  - No specific sub categories
- keys: constants that are used as keys as part of key-value pairs in various maps across the operator 
  - General ArgoCD Keys: Keys that are used for Argo CD wide settings
  - Component specific Keys: Keys used in constants that are specific to a given Argo CD component
  - Domain specific keys: Keys that are mostly used in labels/annotations, and typically follow a `group.domain/label-name` format. We can group all such constants together for ease of access
- names: 
  - names: constants that represent fixed names for resources
  - suffixes: constants that represent suffixes used to make unique/ component specific resources
- values: constants that represent standardized field values in maps and other structs, as well as states
  - no specific sub categories

This structure of constants can be applied to a project wide level, as well as individual component levels. For the project level these constants are defined in separate files in the `common` folder. For individual components they can all be stored in the same `constants.go` file, separated out into these sections.

## Naming convention

- General Argo CD constant names should start with "ArgoCD.." eg: `ArgoCDDefaultLogLevel`
- Component specific constants should start with the component name, e.g: `GrafanaDefaultVersion`
- Domain specific Keys should be named to represent the key itself. For eg:
  - `app.kubernetes.io/name` should be stored in a const named `AppK8sKeyName`
  - `argocd.argoproj.io/test` => `ArgoCDArgoprojKeyTest`
- Environment variable constants should end in `EnvVar`. Eg: `DexImageEnvVar`

General rule of thumb is to make constant names as descriptive as possible (without making them too long) so that it easy to understand what it represents without needing to go to its definition

# File naming conventions

Follow general golang conventions when it comes to naming your files. Some of those include:
- use all lowercase alphabets in filenames
- use `_` if filename contains multiple words
- `_test` is reserved for test files
- avoid using camel case in filenames
- filenames should be short. Filenames should for the most part either be the name of the resource the file is concerned with (e.g `service.go`) or if it is a higher level file in the package, `<package name>.go`, or something else for unique situations
- filenames can (and should) repeat for files dealing with similar responsibilities across different packages, so that a predictable and intuitive file structure is maintained.


# Error handling

- Each error should either be logged or returned. Rarely ever both
- If handling an error in a function that is performing a specific action, annotate the error with context and the name of the function for tracability. eg:

```
	if err != nil {
		errors.Wrapf(err, "reconcileTLSSecret: failed to calculate checksum for %s in namespace %s", common.ArgoCDRedisServerTLSSecretName, rr.Instance.Namespace)
	}
```
  
- If handling an error in a function that just delegates to other functions, no need to add context, as the ground level fn would already have added the required context. Eg:

```
	...

	// reconcile ha role
	if err := rr.reconcileHARole(); err != nil {
		return err
	}

	// reconcile serviceaccount
	if err := rr.reconcileServiceAccount(); err != nil {
		return err
	}

	...
```

- If a function is reconciling a single resource in a single namespace, return errors immediately, as that resource is critical to the component

```
if err = permissions.CreateClusterRole(desiredClusterRole, *acr.Client); err != nil {
			return err
		}
```

- If a function generates a list of independant errors, use the custom defined `util.MultiError` type. Examples for such situations are :
  - A function calls multiple other functions that are independently operating, and each is capable of returning errors. In such cases we should collect errors from each function without blocking subsequent functions by returning at first encountered error
  - A function may run a loop to perform independent actions. In this case as well we want to report all errors back without blocking reconciliation on other resources
 
``` 
func (rr *RedisReconciler) reconcileHAConfigMaps() error {
	var reconErrs util.MultiError

	err := rr.reconcileHAConfigMap()
	reconErrs.Append(err)

	err = rr.reconcileHAHealthConfigMap()
	reconErrs.Append(err)

	return reconErrs.ErrOrNil()
}
```



- At every level decide if encountered error is important enough to return (thereby blocking further execution) :

```
// component-controller level
if acr.ClusterScoped {
		err = acr.reconcileClusterRole()
		if err != nil {
			acr.Logger.Error(err, "error reconciling clusterRole")
			// error in single cluster role can break the instance. Return error
            return err
		}
	}

	if err := acr.reconcileRoles(); err != nil {
		  // error in one role in one namespace doesn't need to block reconciling other app-controller resources. Only Log error 
          acr.Logger.Error(err, "error reconciling roles")
	}

// argocd_controller level
// core components, return reconciliation errors
	if err := r.AppController.Reconcile(); err != nil {
		r.Logger.Error(err, "failed to reconcile application controller")
		return err
	}

// non-core components, don't return reconciliation errors
	if err := r.AppsetController.Reconcile(); err != nil {
		r.Logger.Error(err, "failed to reconcile applicationset controller")
	}
```

# Logging

- Each new controller gets its own logger instance. Instantiated with component name, current instance name and current instance namespace. e.g:

```
acr.Logger = ctrl.Log.WithName(ArgoCDApplicationControllerComponent).WithValues("instance", acr.Instance.Name, "instance-namespace", acr.Instance.Namespace)

```

- Log errors ONLY at the component controller's reconciliation level, or for functions that may be called by external packages 
- Logging and returning erros at each stage spams the output with the same error message logged over and over as it propogates up the call stack

```
if rr.Instance.Spec.HA.Enabled {
		// clean up regular redis resources first
		if err := rr.DeleteNonHAResources(); err != nil {
			rr.Logger.Error(err, "failed to delete non HA redis resources")
		}

		// reconcile HA resources
		if err := rr.reconcileHA(); err != nil {
			rr.Logger.Error(err, "failed to reconcile resources in HA mode")
			return err
		}
	} 
```

- Include function name in error message for improved tracking ONLY when logging errors.  e.g:

```
acr.Logger.Error(err, "reconcileManagedRoles: failed to retrieve role", "name", existingRole.Name, "namespace", existingRole.Namespace)
```

- Use debug level (`Logger.Debug`) when recording non-essential information. i.e, information on events that don't block happy path execution, but can provide hints if troubleshooting is needed e.g:

```
acr.Logger.Debug("reconcileManagedRoles: one or more mutations could not be applied")
acr.Logger.Debug("reconcileManagedRoles: skip reconciliation in favor of custom role", "name", customRoleName)
```

- Use Info level (`Logger.Info`) for all other info-level logs.  Any new action taken by the controller that is critical to normal functioning.
- - No need to mention function names when logging at `info` level. eg:

```
acr.Logger.Info("role created", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
```

- Only use log statements to log success/error if the function belongs to a controller package and is invoked by the controller. No need to log statements from utility/helper packages. e.g:

```
// app-controller package
if err = permissions.CreateRole(desiredRole, *acr.Client); err != nil {
				acr.Logger.Error(err, "failed to create role", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
			}

// permissions package 
func CreateRole(role *rbacv1.Role, client cntrlClient.Client) error {
	// no need to log here
    return client.Create(context.TODO(), role)
}
```


** Update: Jan 27, 2024 **

Some major changes introduced that we should observe going forwards:

- Concurrent development of new operator while leaving existing code and packages untouched for the most part. We want to avoid introducing any changes to existing code as far as possible, to keep things from breaking and make sure the branch always compiles. The only exception is parts of existing code that will carry into the new code base as well (such as changes to argocd-_types.go). Changes to these files are acceptable since they will continue to remain and need to be updated. Avoid changes to files that are definitely going away (such as deployments.go for ex)
  
- Addition of TOBEREMOVED.go in every package
Instead of deleting a piece of code that has been refactored/moved/renamed (to be used by the new code), duplicate that code in the new location and move the existing function/set of functions into that package's TOBEREMOVED.go. This ensures existing code does not break (references remain in tact) and we have a way to track which code has not yet been replicated in the new codebase

- For Argo CD package, a single TOBEREMOVED.go file is not enough. Subsequent component controller development will lead to too many merge conflicts in this file as every branch will try to add new stuff into this file. Each component controller will have its own dedicated TOBEREMOVED.go file, so that concurrent development does not affect other branches, and merge conflicts to be solved in these TOBEREMOVED files are kept to a minimum

- If renaming/moving constants, do the same
move existing constant to TOBEREMOVED and create the renamed constant in the correct location

- If moving/replacing unit tests around, do the same
Move existing unit test to TOBEREMOVED_test.go so we don't lose the existing unit test

- Moving component constants to common package
In order to avoid potential import cycles it might be best to keep all component constants in the common package as oppposed to their own individual packages as was decided before. However, each component gets a dedicated file for it self (appcontroller.go, redis.go etc) so as to still maintain some isolation and organization for those components
In the future maybe we can keep only the publicly needed constants in common for each component but for now let's keep all constants together

- Separation of pkg/argoutil and pkg/util packages
Going forwards, pkg/util is reserved for pure utility functions that are not kubernetes/argocd-operator specific. This includes things like string/bit manipulation, data structure operations etc. Each unique function should be in a dedicated file following existing patterns. For any "utility" functions that are kubernetes/argo-cd/operator specific place them in pkg/argoutil package. This includes things like k8s client/api manipulation/communications, argo-cd admin password/security related stuff etc

- Maintaining both ReconcileArgoCD and ArgoCDReconciler
ArgoCDReconciler is the updated reconciler struct, while ReconcileArgoCD is the old reconciler struct. We should maintain both temporarily, since all of the existing code base requires the continued existence of ReconcileArgoCD, and keeping it around reduces the no. of changes to old code during rebase against master
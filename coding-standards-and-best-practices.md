# General coding standards and practices

- Only provide input parameters that are required by a function, instead of passing the entire custom resource. This allows for easier testing and trims down functions' access to bare minimums
- Make sure a function is not trying to do more than it is supposed to do - remember the package responsibility and decide if a given function should be performing an action or not
- Add brief comments explaining what a function does at the top of each function (use chatGPT if required)
- Coding style should be uniform across the project - if something similar to what you are writing has been done elsewhere, try to stick to the same code format
- Don't hard code any recurring strings- create a new constant for it in the package's `constants.go`
- If adding new functionality in the `argoutil` package - check if the function would fit in any existing file, if not create a new one. It's better to have many focused files within the package (`strings.go` , `maps.go` , `resource.go` etc) than 1 big file called `util.go` with a random assortment of functions in it


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


# Utility functions

- Utility functions are generic go functions that perform low level tasks that are not project specific. These functions likely deal with go data structures or perform generic functions that can be used in any go project.
- Some common examples are string manipulation, merging 2 maps etc.
- All utility functions should not be dumped into a single `util.go` file. Each utility function will have a specific domain that it belongs to, and utility functions of the same domain should be grouped together into a single file
that is representative of this domain, and placed in the existing `argoutil` package. Examples of this are:
	- `string.go` containing string manipulation functions
	- `client.go` containing client generation functions
	- `map.go` containing map manipulation functions
- When adding a new function to `argoutil` package, check if this function fits into any of the existing files. If not, create a new file that will contain this and similar functions moving forward. The goal is to have a flat file structure that has files that group similar utility functions together by domain
- Main rule of thumb to keep in mind when deciding if a given function should be considered a 'utility' function or not is - Does this function care about Argo CD or Argo CD Operator in any way or not. If the answer is yes, it should likely not be in the utility package, as it is operator specific.

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



# Constants

All existing constants can be broken down into the following categories and sub-categories:
- defaults: constants that define fall back values and only get activated if user has not explicitly set a value for a given field 
  - No specific sub categories; defaults can be grouped by function - image/resource constraints etc
- env var: constants that store environment variable values
  - No specific sub categories
- keys: constants that are used as keys as part of key-value pairs in various maps across the operator 
  - General ArgoCD Keys: Keys that are used for Argo CD wide settings
  - Component specific Keys: Keys used in constants that are specific to a given Argo CD component
  - Domain specific keys: Keys that are mostly used in labels/annotations, and typically follow a `group.domain/label-name` format. We can group all such constants
	together for ease of access
- names: constants that represent fixed names for resources
  - names: constants that represent fixed names for resources
  - suffixes: constants that represent suffixes used to make unique/ component specific resources
- values: constants that represent standardized field values in maps and other structs, as well as states
  - no specific sub categories

This structure of constants can be applied to a project wide level, as well as individual component levels. For the project level these constants are defined in separate files in the `common` folder. For individual components they can all be stored in the same `constants.go` file, separeated out into these sections.

## Naming convention

- General Argo CD constant names should start with "ArgoCD.." eg: `ArgoCDDefaultLogLevel`
- Component specific constants should start with the component name, e.g: `GrafanaDefaultVersion`
- Domain specific Keys should be named to represent the key itself. For eg:
  - `app.kubernetes.io/name` should be stored in a const named `AppK8sKeyName`
  - `argocd.argoproj.io/test` => `ArgoCDArgoprojKeyTest`
- Environment variable constants should end in `EnvVar`. Eg: `DexImageEnvVar`

General rule of thumb is to make constant names as descriptive as possible (without making them too long) so that it easy to understand what it represents without needing to go to its definition
# Error handling

- If a function is reconciling a single resource in a single namespace, return errors immediately, as that resource is critical to the component

```
if err = permissions.CreateClusterRole(desiredClusterRole, *acr.Client); err != nil {
			acr.Logger.Error(err, "reconcileClusterRole: failed to create clusterRole")
			return err
		}
```

- If a function is reconciling many instances of a resource across multiple namespaces, or if currently running in a loop, store error but don't break the loop. Return the latest error at the end of the loop (If there are multiple errors, user must solve them one by one from latest to oldest)
 
``` 
 var mutationErr error
	...
	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, role, request.Client)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return role, fmt.Errorf("RequestRole: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}
```

- If logging the error, then just return error directly
```
if err = permissions.CreateClusterRole(desiredClusterRole, *acr.Client); err != nil {
			acr.Logger.Error(err, "reconcileClusterRole: failed to create clusterRole")
			return err
		}
```

- If not logging the error, return error with context for better tracking. If using `fmt.Errorf` use `%w` format specifier for error (see https://stackoverflow.com/questions/61283248/format-errors-in-go-s-v-or-w). e.g:

```
return role, fmt.Errorf("RequestRole: one or more mutation functions could not be applied: %s", mutationErr)

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

- Anytime error is encountered, use `Logger.Error` to record error
- Include function name in error message for improved tracking.  e.g:

```
acr.Logger.Error(err, "reconcileManagedRoles: failed to retrieve role", "name", existingRole.Name, "namespace", existingRole.Namespace)
```
- Use debug level (`Logger.V(1).Info`) when recording non-essential information. i.e, information on events that don't block happy path execution, but can provide hints if troubleshooting is needed e.g:

```
acr.Logger.V(1).Info("reconcileManagedRoles: one or more mutations could not be applied")
acr.Logger.V(1).Info("reconcileManagedRoles: skip reconciliation in favor of custom role", "name", customRoleName)
```

- Use Info level (`Logger.Info` or `Logger.V(0).Info`) for all other info-level logs.  Any new action taken by the controller that is critical to normal functioning. e.g:

```
acr.Logger.V(0).Info("reconcileManagedRoles: role created", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
```

- Only use log statements to log success/error if the function belongs to a controller package and is invoked by the controller. No need to log statements from utility/helper packages. e.g:

```
// app-controller package
if err = permissions.CreateRole(desiredRole, *acr.Client); err != nil {
				acr.Logger.Error(err, "reconcileManagedRoles: failed to create role", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
			}

// permissions package 
func CreateRole(role *rbacv1.Role, client ctrlClient.Client) error {
	// no need to log here
    return client.Create(context.TODO(), role)
}
```
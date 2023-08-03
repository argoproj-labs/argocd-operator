# General coding standards and practices

- Only provide input parameters that are required by a function, instead of passing the entire custom resource. This allows for easier testing and trims down functions' access to bare minimums
- Make sure a function is not trying to do more than it is supposed to do - remember the package responsibility and decide if a given function should be performing an action or not
- Add brief comments explaining what a function does at the top of each function (use chatGPT if required)
- Coding style should be uniform across the project - if something similar to what you are writing has been done elsewhere, try to stick to the same code format
- Don't hard code any recurring strings- create a new constant for it in the package's `constants.go`
- If adding new functionality in the `argoutil` package - check if the function would fit in any existing file, if not create a new one. It's better to have many focused files within the package (`strings.go` , `maps.go` , `resource.go` etc) than 1 big file called `util.go` with a random assortment of functions in it


# Controller packages & testing

- Each package must have its own constants.go if we define any package/controller specific constants
- Each package must have its own `util.go / helper.go` to contain functions related to the package but not directly to the reconciliation of a particular resource
- `constants.go` should be split into the following sections:
  - `defaults` - any const that stores default/fallback values for a parameter that the package/controller is defined by
  - `keys` - standardized keys for various map entries
  - `values` - standardized values for standardized keys
  - `names` - names of controller-specific resources
  - `miscellaneous`
- Each managed resource's reconcile function should be in a dedicated file for that resource type
- Each package should define common testing variables/functions in the `<package_name>_test.go` file, which should be accessed across the package

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
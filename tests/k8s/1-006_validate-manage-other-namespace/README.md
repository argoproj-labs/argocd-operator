# manage-other-namespace

## Synopsis

This is a test for the `managed-by` permission extension. It creates another
namespace, labels it and checks whether all roles and rolebindings are setup.
Then, it will sync an application into that namespace.

## Success criterias

* Role and RoleBindings are created succesfully
* Application is correctly synced to target namespace
* After unlabeling, this is not possible anymore

## Remarks
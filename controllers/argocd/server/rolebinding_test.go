package server

//unc TestServerReconciler_createResetAndDeleteRoleBinding(t *testing.T) {
//	ns := argocdcommon.MakeTestNamespace()
//	sr := makeTestServerReconciler(t, ns)
//
//	resourceName = testResourceName
//	resourceLabels = testResourceLabels
//	testRoleRef := rbacv1.RoleRef{
//		APIGroup: rbacv1.GroupName,
//		Kind:     common.RoleKind,
//		Name:     resourceName,
//	}
//
//	// create sa & role
//	err := sr.reconcileServiceAccount()
//	assert.NoError(t, err)
//	err = sr.reconcileRole()
//	assert.NoError(t, err)
//
//	// create rolebinding
//	err = sr.reconcileRoleBinding()
//	assert.NoError(t, err)
//
//	// rolebinding should be created
//	currentRoleBinding := &rbacv1.RoleBinding{}
//	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: testResourceName, Namespace: argocdcommon.TestNamespace}, currentRoleBinding)
//	assert.NoError(t, err)
//	assert.Equal(t, testResourceLabels, currentRoleBinding.Labels)
//	assert.Equal(t, testRoleRef, currentRoleBinding.RoleRef)
//
//	// modify rolebinding
//	currentRoleBinding.RoleRef = rbacv1.RoleRef{}
//	err = sr.Client.Update(context.TODO(), currentRoleBinding)
//	assert.NoError(t, err)
//
//	err = sr.reconcileRoleBinding()
//	assert.NoError(t, err)
//
//	// rolebinding should reset to default
//	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: testResourceName, Namespace: argocdcommon.TestNamespace}, currentRoleBinding)
//	assert.NoError(t, err)
//	assert.Equal(t, testRoleRef, currentRoleBinding.RoleRef)
//
//	// delete rolebinding
//	err = sr.deleteRoleBinding(testResourceName, sr.Instance.Namespace)
//	assert.NoError(t, err)
//
//	// rolebinding should not exist
//	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: testResourceName, Namespace: argocdcommon.TestNamespace}, currentRoleBinding)
//	assert.Equal(t, true, errors.IsNotFound(err))
//
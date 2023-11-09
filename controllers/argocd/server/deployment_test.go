package server

//func TestServerReconciler_createUpdateAndDeleteDeployment(t *testing.T) {
//	ns := argocdcommon.MakeTestNamespace()
//	sr := makeTestServerReconciler(t, ns)
//
//	resourceName = testResourceName
//	resourceLabels = testResourceLabels
//
//	// reconcile sa, role & rolebinding
//	err := sr.reconcileServiceAccount()
//	assert.NoError(t, err)
//	err = sr.reconcileRole()
//	assert.NoError(t, err)
//	err = sr.reconcileRoleBinding()
//	assert.NoError(t, err)
//
//	// reconcile deployment
//	err = sr.reconcileDeployment()
//	assert.NoError(t, err)
//
//	// deployment should be created
//	currentDeployment := &appsv1.Deployment{}
//	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: testResourceName, Namespace: argocdcommon.TestNamespace}, currentDeployment)
//	assert.NoError(t, err)
//	assert.Equal(t, testResourceLabels, currentDeployment.Labels)
//	//assert.Equal(t, testRoleRef, currentRoleBinding.RoleRef)
////
//	//// modify rolebinding
//	//currentRoleBinding.RoleRef = rbacv1.RoleRef{}
//	//err = sr.Client.Update(context.TODO(), currentRoleBinding)
//	//assert.NoError(t, err)
////
//	//err = sr.reconcileRoleBinding()
//	//assert.NoError(t, err)
////
//	//// rolebinding should reset to default
//	//err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: testResourceName, Namespace: argocdcommon.TestNamespace}, currentRoleBinding)
//	//assert.NoError(t, err)
//	//assert.Equal(t, testRoleRef, currentRoleBinding.RoleRef)
////
//	//// delete rolebinding
//	//err = sr.deleteRoleBinding(testResourceName, sr.Instance.Namespace)
//	//assert.NoError(t, err)
////
//	//// rolebinding should not exist
//	//err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: testResourceName, Namespace: argocdcommon.TestNamespace}, currentRoleBinding)
//	//assert.Equal(t, true, errors.IsNotFound(err))
//}
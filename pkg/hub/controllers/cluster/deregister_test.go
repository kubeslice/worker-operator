package cluster

import (
	"runtime/debug"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

var testClusterRoleRef = rbacv1.RoleRef{
	APIGroup: "rbac.authorization.k8s.io",
	Kind:     "ClusterRole",
	Name:     clusterRoleName,
}

var testClusterRoleBindingSubject = []rbacv1.Subject{{
	Kind:      "ServiceAccount",
	Name:      serviceAccountName,
	Namespace: ControlPlaneNamespace,
}}

func TestConstructClusterRoleBinding(t *testing.T) {
	crb := constructClusterRoleBinding()
	AssertEqual(t, crb.Name, clusterRoleBindingName)
	AssertEqual(t, crb.RoleRef, testClusterRoleRef)
	AssertEqual(t, len(crb.Subjects), 1)
	AssertEqual(t, crb.Subjects[0], testClusterRoleBindingSubject[0])
}

func TestConstructConfigMap(t *testing.T) {
	data := "this is the data."
	cm := constructConfigMap(data)
	AssertEqual(t, cm.Data["kubeslice-cleanup.sh"], data)
}

func AssertEqual(t *testing.T, actual interface{}, expected interface{}) {
	t.Helper()
	if actual != expected {
		t.Log("expected --", expected, "actual --", actual)
		t.Fail()
	}
}

func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected No Error but got %s, Stack:\n%s", err, string(debug.Stack()))
	}
}

package cluster

import (
	"context"
	"fmt"
	"testing"

	"github.com/dailymotion/allure-go"

	hubv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	mevents "github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/worker-operator/controllers"
	ossEvents "github.com/kubeslice/worker-operator/events"
	utilMock "github.com/kubeslice/worker-operator/pkg/mock"
)

func TestDeregisterSuite(t *testing.T) {
	for k, v := range DeregisterTestBed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var DeregisterTestBed = map[string]func(*testing.T){
	"TestDeregisterCreateConfigMap": testCreateConfigMap,
	// "TestDeregisterCreateDeregisterJob": testCreateDeregisterJob,
}

func testCreateConfigMap(t *testing.T) {
	data := "this is the data."
	cm := constructConfigMap(data)
	AssertEqual(t, cm.Data["kubeslice-cleanup.sh"], data)
}

func TestCreateDeregisterJob(t *testing.T) {
	ctx, client, r := setupDeregisterTest("test", "test")
	cluster := hubv1alpha1.Cluster{}

	client.On("Get", ctx, "test", &cluster).Return(nil)
	err := r.createDeregisterJob(ctx, &cluster)
	if err != nil {
		fmt.Println(err)
	}
	// TODO: IMPLEMENT
	client.AssertExpectations(t)
}

func setupDeregisterTest(name string, project string) (context.Context, *utilMock.MockClient, *Reconciler) {
	clientMock := &utilMock.MockClient{}

	ctx := context.Background()
	testClusterEventRecorder := mevents.NewEventRecorder(clientMock, nil, ossEvents.EventsMap, mevents.EventRecorderOptions{
		Cluster:   name,
		Project:   project,
		Component: "worker-operator",
		Namespace: controllers.ControlPlaneNamespace,
	})
	clusterReconciler := &Reconciler{
		clientMock,
		clientMock,
		testClusterEventRecorder,
	}
	return ctx, clientMock, clusterReconciler
}

package controller

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	systemautoscaler "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned/fake"
	informers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t *testing.T

	client     *fake.Clientset
	kubeclient *k8sfake.Clientset
	// Objects to put in the store.
	slaLister       []*systemautoscaler.ServiceLevelAgreement
	podScalesLister []*systemautoscaler.PodScale
	servicesLister  []*corev1.Service
	podLister       []*corev1.Pod
	// Actions expected to happen on the client.
	kubeactions []core.Action
	actions     []core.Action
	// Objects from here preloaded into NewSimpleFake.
	kubeobjects []runtime.Object
	objects     []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeobjects = []runtime.Object{}
	return f
}

func newSLA(name string, labels map[string]string) *systemautoscaler.ServiceLevelAgreement {
	return &systemautoscaler.ServiceLevelAgreement{
		TypeMeta: metav1.TypeMeta{APIVersion: systemautoscaler.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: systemautoscaler.ServiceLevelAgreementSpec{
			ServiceSelector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
	}
}

func newApplication(name string, labels map[string]string) (*corev1.Service, *corev1.Pod) {
	podLabels := map[string]string{
		"match": "bar",
	}
	return &corev1.Service{
			TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "services"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
				Labels:    labels,
			},
			Spec: corev1.ServiceSpec{
				Selector: podLabels,
			},
		}, &corev1.Pod{
			TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "pods"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foobar",
				Namespace: metav1.NamespaceDefault,
				Labels:    podLabels,
			},
		}
}

func (f *fixture) newController() (*Controller, informers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.client = fake.NewSimpleClientset(f.objects...)
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)

	i := informers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	c := NewController(f.kubeclient,
		f.client,
		i.Systemautoscaler().V1beta1().ServiceLevelAgreements(),
		i.Systemautoscaler().V1beta1().PodScales(),
		k8sI.Core().V1().Services(),
		k8sI.Core().V1().Pods())

	c.slasSynced = alwaysReady
	c.podScalesSynced = alwaysReady
	c.servicesSynced = alwaysReady
	c.podSynced = alwaysReady
	c.recorder = &record.FakeRecorder{}

	for _, f := range f.slaLister {
		_ = i.Systemautoscaler().V1beta1().ServiceLevelAgreements().Informer().GetIndexer().Add(f)
	}

	for _, f := range f.podScalesLister {
		_ = i.Systemautoscaler().V1beta1().PodScales().Informer().GetIndexer().Add(f)
	}

	for _, f := range f.servicesLister {
		_ = k8sI.Core().V1().Services().Informer().GetIndexer().Add(f)
	}

	for _, f := range f.podLister {
		_ = k8sI.Core().V1().Pods().Informer().GetIndexer().Add(f)
	}

	return c, i, k8sI
}

func (f *fixture) run(fooName string) {
	f.runController(fooName, true, false)
}

func (f *fixture) runExpectError(fooName string) {
	f.runController(fooName, true, true)
}

func (f *fixture) runController(fooName string, startInformers bool, expectError bool) {
	c, i, k8sI := f.newController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		i.Start(stopCh)
		k8sI.Start(stopCh)
	}

	err := c.syncServiceLevelAgreement(fooName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing foo: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing foo, got nil")
	}

	actions := filterInformerActions(f.client.Actions())
	for i, action := range actions {
		if len(f.actions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.actions), actions[i:])
			break
		}

		expectedAction := f.actions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.actions) > len(actions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.actions)-len(actions), f.actions[len(actions):])
	}

	k8sActions := filterInformerActions(f.kubeclient.Actions())
	for i, action := range k8sActions {
		if len(f.kubeactions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(k8sActions)-len(f.kubeactions), k8sActions[i:])
			break
		}

		expectedAction := f.kubeactions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.kubeactions) > len(k8sActions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeactions)-len(k8sActions), f.kubeactions[len(k8sActions):])
	}
}

// checkAction verifies that expected and actual actions are equal and both have
// same attached resources
func checkAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.CreateActionImpl:
		e, _ := expected.(core.CreateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.DeleteActionImpl:
		e, _ := expected.(core.DeleteActionImpl)

		if e.GetName() != a.GetName() || e.GetNamespace() != a.GetNamespace() {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(e, a))
		}
	case core.UpdateActionImpl:
		e, _ := expected.(core.UpdateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.PatchActionImpl:
		e, _ := expected.(core.PatchActionImpl)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, patch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expPatch, patch))
		}
	default:
		t.Errorf("Uncaptured Action %s %s, you should explicitly add a case to capture it",
			actual.GetVerb(), actual.GetResource().Resource)
	}
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	var ret []core.Action
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "podscales") ||
				action.Matches("watch", "podscales") ||
				action.Matches("list", "servicelevelagreements") ||
				action.Matches("watch", "servicelevelagreements") ||
				action.Matches("list", "pods") ||
				action.Matches("watch", "pods") ||
				action.Matches("list", "services") ||
				action.Matches("watch", "services")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

func (f *fixture) expectCreatePodScaleAction(p *systemautoscaler.PodScale) {
	f.actions = append(f.actions, core.NewCreateAction(
		schema.GroupVersionResource{
			Group:    "systemautoscaler.polimi.it",
			Version:  "v1beta1",
			Resource: "podscales",
		},
		p.Namespace,
		p))
}

func (f *fixture) expectDeletePodScaleAction(p *systemautoscaler.PodScale) {
	f.actions = append(f.actions, core.NewDeleteAction(
		schema.GroupVersionResource{
			Group:    "systemautoscaler.polimi.it",
			Version:  "v1beta1",
			Resource: "podscales",
		},
		p.Namespace,
		p.Name))
}

func (f *fixture) expectUpdateServiceAction(svc *corev1.Service) {
	f.kubeactions = append(f.kubeactions, core.NewUpdateAction(
		schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "services"},
		svc.Namespace,
		svc))
}

func getKey(foo *systemautoscaler.ServiceLevelAgreement, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(foo)
	if err != nil {
		t.Errorf("Unexpected error getting key for foo %v: %v", foo.Name, err)
		return ""
	}
	return key
}

// TestCreatePodScale checks that a Pod matching a SLA will get its PodScale
func TestCreatePodScale(t *testing.T) {
	f := newFixture(t)

	labels := map[string]string{
		"app": "foo",
	}

	sla := newSLA("foo-sla", labels)
	svc, pod := newApplication("foo-app", labels)
	expectedPodScale := NewPodScale(pod, sla, svc.Spec.Selector)

	f.slaLister = append(f.slaLister, sla)
	f.servicesLister = append(f.servicesLister, svc)
	f.podLister = append(f.podLister, pod)

	f.objects = append(f.objects, sla)
	f.kubeobjects = append(f.kubeobjects, svc)
	f.kubeobjects = append(f.kubeobjects, pod)

	f.expectCreatePodScaleAction(expectedPodScale)
	f.expectUpdateServiceAction(svc)

	f.run(getKey(sla, t))
}

// TestCreatePodScale checks that a Pod matching a SLA will get its PodScale
func TestDoNothing(t *testing.T) {
	f := newFixture(t)

	labels := map[string]string{
		"app": "foo",
	}

	sla := newSLA("foo-sla", labels)
	svc, pod := newApplication("foo-app", labels)
	podscale := NewPodScale(pod, sla, svc.Spec.Selector)

	f.slaLister = append(f.slaLister, sla)
	f.servicesLister = append(f.servicesLister, svc)
	f.podLister = append(f.podLister, pod)
	f.podScalesLister = append(f.podScalesLister, podscale)

	f.objects = append(f.objects, sla)
	f.objects = append(f.objects, podscale)
	f.kubeobjects = append(f.kubeobjects, svc)
	f.kubeobjects = append(f.kubeobjects, pod)

	f.expectUpdateServiceAction(svc)

	f.run(getKey(sla, t))
}

// TestServiceSelectorChange checks the PodScale cleanup when the SLA's ServiceSelector changes.
func TestServiceSelectorChange(t *testing.T) {
	f := newFixture(t)

	oldServiceSelector := map[string]string{
		"app": "foo",
	}

	sla := newSLA("foo-sla", oldServiceSelector)
	matchedSvc, matchedPod := newApplication("foo-app", oldServiceSelector)
	matchedPodScale := NewPodScale(matchedPod, sla, matchedSvc.Spec.Selector)

	newServiceSelector := map[string]string{
		"app": "bar",
	}

	sla.Spec.ServiceSelector = &metav1.LabelSelector{
		MatchLabels:      newServiceSelector,
	}

	matchedSvc.Labels[SubjectToLabel] = sla.Name

	f.slaLister = append(f.slaLister, sla)
	f.servicesLister = append(f.servicesLister, matchedSvc)
	f.podLister = append(f.podLister, matchedPod)
	f.podScalesLister = append(f.podScalesLister, matchedPodScale)

	f.objects = append(f.objects, sla)
	f.objects = append(f.objects, matchedPodScale)
	f.kubeobjects = append(f.kubeobjects, matchedSvc)
	f.kubeobjects = append(f.kubeobjects, matchedPod)

	f.expectDeletePodScaleAction(matchedPodScale)

	f.run(getKey(sla, t))
}
// TestServiceSelectorChange checks the PodScale cleanup when the SLA's ServiceSelector changes.
func TestReplicaIncrease(t *testing.T) {
	f := newFixture(t)

	oldServiceSelector := map[string]string{
		"app": "foo",
	}

	sla := newSLA("foo-sla", oldServiceSelector)
	matchedSvc, matchedPod := newApplication("foo-app", oldServiceSelector)
	matchedPodScale := NewPodScale(matchedPod, sla, matchedSvc.Spec.Selector)

	matchedSvc.Labels[SubjectToLabel] = sla.Name

	newPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "pods"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foobarfoo",
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				"match": "bar",
			},
		},
	}

	expectedPodScale := NewPodScale(newPod, sla, matchedSvc.Spec.Selector)

	f.slaLister = append(f.slaLister, sla)
	f.servicesLister = append(f.servicesLister, matchedSvc)
	f.podLister = append(f.podLister, matchedPod)
	f.podLister = append(f.podLister, newPod)
	f.podScalesLister = append(f.podScalesLister, matchedPodScale)

	f.objects = append(f.objects, sla)
	f.objects = append(f.objects, matchedPodScale)
	f.kubeobjects = append(f.kubeobjects, matchedSvc)
	f.kubeobjects = append(f.kubeobjects, matchedPod)
	f.kubeobjects = append(f.kubeobjects, newPod)

	f.expectCreatePodScaleAction(expectedPodScale)
	f.expectUpdateServiceAction(matchedSvc)

	f.run(getKey(sla, t))
}

// TestServiceSelectorChange checks the PodScale cleanup when the SLA's ServiceSelector changes.
func TestReplicaDecrease(t *testing.T) {
	f := newFixture(t)

	oldServiceSelector := map[string]string{
		"app": "foo",
	}

	sla := newSLA("foo-sla", oldServiceSelector)
	matchedSvc, matchedPod := newApplication("foo-app", oldServiceSelector)
	matchedPodScale := NewPodScale(matchedPod, sla, matchedSvc.Spec.Selector)

	matchedSvc.Labels[SubjectToLabel] = sla.Name

	newPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "pods"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foobarfoo",
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				"match": "bar",
			},
		},
	}

	expectedPodScale := NewPodScale(newPod, sla, matchedSvc.Spec.Selector)

	f.slaLister = append(f.slaLister, sla)
	f.servicesLister = append(f.servicesLister, matchedSvc)
	f.podLister = append(f.podLister, matchedPod)
	f.podScalesLister = append(f.podScalesLister, matchedPodScale)
	f.podScalesLister = append(f.podScalesLister, expectedPodScale)

	f.objects = append(f.objects, sla)
	f.objects = append(f.objects, matchedPodScale)
	f.objects = append(f.objects, expectedPodScale)
	f.kubeobjects = append(f.kubeobjects, matchedSvc)
	f.kubeobjects = append(f.kubeobjects, matchedPod)

	f.expectDeletePodScaleAction(expectedPodScale)
	f.expectUpdateServiceAction(matchedSvc)

	f.run(getKey(sla, t))
}

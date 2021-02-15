package replicaupdater

import (
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestGetReplicaSetNameFromPod(t *testing.T) {
	fooPod := corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			OwnerReferences: []v1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "foo",
				},
			},
		},
	}

	barPod := corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			OwnerReferences: []v1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "bar",
				},
			},
		},
	}

	errPod := corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			OwnerReferences: []v1.OwnerReference{
				{
					Kind: "Deployment",
					Name: "bar",
				},
			},
		},
	}

	testcases := []struct {
		description string
		pod         corev1.Pod
		expected    string
		error       bool
	}{
		{
			description: "find replicaset of foopod",
			pod:         fooPod,
			expected:    "foo",
			error:       false,
		},
		{
			description: "find replicaset of foopod",
			pod:         barPod,
			expected:    "bar",
			error:       false,
		},
		{
			description: "find replicaset of foopod",
			pod:         errPod,
			expected:    "",
			error:       true,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual, err := getReplicaSetNameFromPod(&tt.pod)
			if tt.error {
				require.Error(t, err)
			} else {
				require.Equal(t, tt.expected, actual)
			}
		})
	}
}

func TestDeploymentNameFromReplicaSet(t *testing.T) {
	fooRS := appsv1.ReplicaSet{
		ObjectMeta: v1.ObjectMeta{
			OwnerReferences: []v1.OwnerReference{
				{
					Kind: "Deployment",
					Name: "foo",
				},
			},
		},
	}

	barRS := appsv1.ReplicaSet{
		ObjectMeta: v1.ObjectMeta{
			OwnerReferences: []v1.OwnerReference{
				{
					Kind: "Deployment",
					Name: "bar",
				},
			},
		},
	}

	errRS := appsv1.ReplicaSet{
		ObjectMeta: v1.ObjectMeta{
			OwnerReferences: []v1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "bar",
				},
			},
		},
	}

	testcases := []struct {
		description string
		replicaSet  appsv1.ReplicaSet
		expected    string
		error       bool
	}{
		{
			description: "find replicaset of foopod",
			replicaSet:  fooRS,
			expected:    "foo",
			error:       false,
		},
		{
			description: "find replicaset of foopod",
			replicaSet:  barRS,
			expected:    "bar",
			error:       false,
		},
		{
			description: "find replicaset of foopod",
			replicaSet:  errRS,
			expected:    "",
			error:       true,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual, err := getDeploymentNameFromReplicaSet(&tt.replicaSet)
			if tt.error {
				require.Error(t, err)
			} else {
				require.Equal(t, tt.expected, actual)
			}
		})
	}
}

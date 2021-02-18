package replicaupdater

import (
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func getReplicaSetNameFromPod(pod *corev1.Pod) (string, error) {
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "ReplicaSet" {
			return owner.Name, nil
		}
	}
	return "", fmt.Errorf("no replicaset found in the pod")
}

func getDeploymentNameFromReplicaSet(replicaSet *appsv1.ReplicaSet) (string, error) {
	for _, owner := range replicaSet.OwnerReferences {
		if owner.Kind == "Deployment" {
			return owner.Name, nil
		}
	}
	return "", fmt.Errorf("no deployment found in the replicaSet")
}

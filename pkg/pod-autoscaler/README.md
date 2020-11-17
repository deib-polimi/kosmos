# Pod Autoscaler
The pod autoscaler is the component that manages the scaling of pods within a node.

# Recommender
The Recommender is the controller that periodically suggests the amount of resources to assign to each pod in order to meet the service level agreement assigned to the service.

It takes as input three types of resources, which are:
- `Pod Scale` to retrieve the actual amount of resources (CPU and memory) assigned to a pod.
- `Custom Metric` to retrieve the actual performance of the pod.
- `Service Level Agreement` to retrieve the desired performance of the pod.

and it outputs:
- `Pod Scale` to set the desired amount of resources (CPU and memory) assigned to a pod.

The Recommender is based on control theory, and it allows to rapidly change the amount of resources of a node in order to meet the service level agreement desired.

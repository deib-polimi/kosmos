# PodScale Controller

PodScale Controller manages `PodScale` resources lifecycle, dealing with their creation and deletion. 

## PodScale lifecyle

Once a new `ServiceLevelAgreement` is deployed into a namespace, the controller will try to find a set of `Services` compatible with the `serviceSelector` and will create a new `PodScale` for each `Pod`. The match is currently done by setting the `MatchLabels` field inside the Selector but a further analysis has to be done regarding the `Selector` strategy since the `MatchExpressions` will not be used.  
After the `PodScale` creation, the controller will try to keep the set of `PodScale` up to date with `Pod` resources, handling changes in the number of replicas and `Pod` deletions. What is not covered at the moment is specified in this [issue] (https://github.com/lterrac/system-autoscaler/issues/2).  
When the `ServiceLevelAgreement` is deleted from the namespace, all the `PodScale` resources generated from it will be also deleted, leaving the namespace as it was before introducing the Agreement.

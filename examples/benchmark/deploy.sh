kubectl delete -f application
kubectl delete -f metrics
kubectl delete -f system-autoscaler

kubectl apply -f application
kubectl apply -f metrics
kubectl apply -f system-autoscaler

kubectl cp kube-system/pod-autoscaler-578b5988c8-52brq:var/containerscale.json containerscale.json -c pod-autoscaler
kubectl exec postgres-statefulset-0 -- psql -d awesomedb -U amazinguser -c "\copy response_information to /response.csv delimiter ',' csv header;"
kubectl cp postgres-statefulset-0:response.csv response.csv

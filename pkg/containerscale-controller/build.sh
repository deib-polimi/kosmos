make build
docker build -t pentabanana/recommender:latest .
docker push pentabanana/recommender:latest

kubectl delete -f deployment.yaml
kubectl apply -f deployment.yaml

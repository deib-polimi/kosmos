make build
docker build -t pentabanana/pod-replicas-updater .
#docker run -it --rm --name prime-numbers prime-numbers
docker push pentabanana/pod-replicas-updater

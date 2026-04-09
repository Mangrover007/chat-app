# Project setup

To get the app running, ensure you have the following installed
by running the given commands.

1. go
```bash
go version
# go version go1.26.0-X:nodwarf5 linux/amd64
```

2. postgresql
```bash
psql --version
# psql (PostgreSQL) 18.2
```

3. redis
```bash
redis-cli -v
# valkey-cli 8.1.4 (git:5f4bae3e-dirty)
```

4. docker
```bash
docker version
# Client:
#  Version:           29.2.1
#  API version:       1.53
#  Go version:        go1.25.6 X:nodwarf5
#  Git commit:        a5c7197d72
#  Built:             Thu Feb  5 10:59:55 2026
#  OS/Arch:           linux/amd64
#  Context:           default

# Server: Docker Engine - Community
#  Engine:
#   Version:          29.2.0
#   API version:      1.53 (minimum version 1.44)
#   Go version:       go1.25.6
#   Git commit:       9c62384
#   Built:            Mon Jan 26 19:26:07 2026
#   OS/Arch:          linux/amd64
#   Experimental:     false
#  containerd:
#   Version:          v2.2.1
#   GitCommit:        dea7da592f5d1d2b7755e3a161be07f43fad8f75
#  runc:
#   Version:          1.3.4
#   GitCommit:        v1.3.4-0-gd6d73eb8
#  docker-init:
#   Version:          0.19.0
#   GitCommit:        de40ad0
```

Optionally, this project can also be run on a kubernetes cluster.
For `minikube`, set up is as follows:

1. Ensure minikube is up and running by running the following
command:
```bash
minikube status
# minikube
# type: Control Plane
# host: Running
# kubelet: Running
# apiserver: Running
# kubeconfig: Configured
```

2. SSH into minikube and run the commands below:
```bash
# ssh into minikube
minikube ssh

# make a directory at root
sudo mkdir -p /db/psql

# exit ssh
exit
```

3. Ensure app's docker images are visible to the minikube docker
environment:
```bash
eval $(minikube docker-env)

# build the image from the dockerfile in /app and in /db
docker build -t chat-ms app/
docker build -t postgres-custom db/

# ensure both images are on minikube's docker environment
docker images

# example output
# chat-ms:latest                                    dfa5e1cef3f9       16.7MB             0B    U
# postgres-custom:latest                            d8a5e98927d8        456MB             0B    U
```

# Running the project
## Locally
1. Ensure `discordv2` DB exists. You can check by logging in to 
psql and running `\l`:
```bash
sudo -iu postgres

psql

\l

# ensure discordv2 is listed as one of the DBs. if not, run:
CREATE DATABASE discordv2;

# exit from postgres
```

Change `PSQL_URI` in `main.go`, or export the URI from shell.

2. Apply DB schema from shell:
```bash
psql -U postgres -d discordv2 -f db/schema.sql
```

3. Ensure all environment variables are set up:

- PSQL_URI
- RDB_URI
- RDB_PASSWORD
- POD_UID (optional)

You can export them from the shell or change them in `main.go`.

For example:
```bash
export PSQL_URI="postgres://postgres:password@127.0.0.1:5432/discordv2"
export RDB_URI="127.0.0.1:6379"
export RDB_PASSWORD=""
```

4. Build and run the binary:
```bash
cd app/
go build -o chat-app cmd/server/main.go

# run the binary
./chat-app

# example output
# ./chat-app 
# 2026/04/07 01:48:29 Server started, listening on port 8080
# 2026/04/07 01:48:29 Pod ID: s1
```

## Minikube

If you want to run it on minikube, follow the instructions below:

1. Start `postgres` and `redis` pods with
```bash
kubectl apply -f postgres.yaml
kubectl apply -f redis-store.yaml
```

Ensure both are up and running:
```bash
kubectl get all

# ensure these two services are present, and their corresponding
# pods are running

# services:
# NAME                       TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)    AGE
# service/postgres-service   ClusterIP   10.107.200.25    <none>        5432/TCP   4d9h
# service/redis-service      ClusterIP   10.100.246.164   <none>        6379/TCP   4d10h

# pods:
# NAME                               READY   STATUS    RESTARTS      AGE
# pod/postgres-85d48bd5df-7qrzp      1/1     Running   5 (25h ago)   4d9h
# pod/redis-store-6559fb9469-7h2h2   1/1     Running   5 (25h ago)   4d10h
```

2. Apply the `chat-ms` config from `/components`:
```bash
kubectl apply -f components/
```

3. Don't forget to port-forward the `chat-ms` deployment so that
you can access it from 127.0.0.1 host. By default, the app runs
on port `8080`:
```bash
kubectl port-forward deploy/chat-ms 8080:8080
```

4. Optionally, you can change the number of replicas of `chat-ms`
from `components/chat-ms.yaml` to 1 for predictable websocket
connections. To do this, change `.spec.replicas` from `3` to `1`.

5. To read logs from any pod, do the following:
```bash
# get the pod ID
kubectl get pods

# example output
# NAME                           READY   STATUS    RESTARTS      AGE
# chat-ms-7795c88c54-6pct6       1/1     Running   3 (32m ago)   26h

# read logs, or follow logs with -f flag
kubectl logs [-f] chat-ms-7795c88c54-6pct6

# example output
# kubectl logs -f chat-ms-7795c88c54-6pct6
# 2026/04/06 19:41:40 Server started, listening on port 8080
# 2026/04/06 19:41:40 Pod ID: 6777bc41-652c-4def-9b81-a97b4386f140
```

# Testing
There is a test script writting in python. If you want to run it,
ensure you have python installed, then run the test:
```bash
python tests/main.py
```

# API endpoints

__BASE URI: `http://127.0.0.1:8080`__

| Method | Endpoint                           | Description                                      |
|--------|------------------------------------|--------------------------------------------------|
| POST   | `/api/register`                    | Register a new user                              |
| POST   | `/api/{guild_id}`                  | Join a guild                                     |
| POST   | `/api/{guild_id}/{channel_id}`     | Send a message to a channel in a guild           |
| GET    | `/ws/`                             | Establish initial WebSocket connection           |
| GET    | `/ws/{guild_id}/{channel_id}`      | Switch active guild/channel for the connection   |

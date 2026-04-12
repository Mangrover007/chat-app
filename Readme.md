# Project Setup

## Prerequisites

Install the following tools and verify installation.

### 1. Go
Installation: https://go.dev/doc/install

```bash
go version
# expected: go version go1.26.x <os>/<arch>
```

---

### 2. PostgreSQL
Installation: https://www.postgresql.org/download/

```bash
psql --version
# expected: psql (PostgreSQL) 18.x
```

---

### 3. Redis
Installation: https://redis.io/docs/latest/operate/oss_and_stack/install/install-redis/

```bash
redis-cli -v
# expected: redis-cli x.x.x
```

---

### 4. Docker (Optional, required for Kubernetes)
Installation: https://docs.docker.com/get-docker/

```bash
docker version
# expected:
# Client:
#  Version:           29.2.1
#  API version:       1.53
#  Go version:        go1.25.6 X:nodwarf5
#  Git commit:        a5c7197d72
#  Built:             Thu Feb  5 10:59:55 2026
#  OS/Arch:           linux/amd64
#  Context:           default

# Server:
#  Engine:
#   Version:          29.2.1
#   API version:      1.53 (minimum version 1.44)
#   Go version:       go1.25.6 X:nodwarf5
#   Git commit:       6bc6209b88
#   Built:            Thu Feb  5 10:59:55 2026
#   OS/Arch:          linux/amd64
#   Experimental:     false
#  containerd:
#   Version:          v2.2.1
#   GitCommit:        dea7da592f5d1d2b7755e3a161be07f43fad8f75.m
#  runc:
#   Version:          1.4.0
#   GitCommit:        
#  docker-init:
#   Version:          0.19.0
#   GitCommit:        de40ad0
```

---

### 5. Minikube (Optional)
- Minikube docs: https://minikube.sigs.k8s.io/docs/start/
- Kubernetes docs: https://kubernetes.io/docs/home/

> Note: Docker must be installed before using Minikube.

---

## Minikube Setup (Optional)

### 1. Verify Minikube is running

```bash
minikube start
minikube status
# expected:
# minikube
# type: Control Plane
# host: Running
# kubelet: Running
# apiserver: Running
# kubeconfig: Configured
```

---

### 2. Prepare filesystem inside Minikube

SSH into minikube docker container and make `/db/psql` directory
at the filesystem root.

```bash
minikube ssh
sudo mkdir -p /db/psql
exit
```

---

### 3. Use Minikube Docker environment

```bash
# point your shell to minikube's internal docker daemon
eval $(minikube docker-env)
```

What this does:
- Redirects all `docker` commands to **Minikube’s internal Docker daemon**
- Required so Kubernetes can see locally built images
- Without this, your images will NOT be available inside the cluster

---

### Build Images

```bash
# build backend image from Dockerfile in app/
docker build -t chat-ms app/

# build custom postgres image from Dockerfile in db/
docker build -t postgres-custom db/
```

About the custom PostgreSQL image:

- The official PostgreSQL Docker image automatically runs any `.sql`, `.sql.gz`, or `.sh` files placed in:
  ```
  /docker-entrypoint-initdb.d/
  ```
  during **initial database creation**

- This is how the schema in `db/schema.sql` gets applied automatically on container startup

- Official documentation:
  https://hub.docker.com/_/postgres

- Direct reference (Initialization scripts section):
  https://hub.docker.com/_/postgres#initialization-scripts

> [!NOTE]
> - Scripts run **only on first initialization** (i.e., when the data directory is empty)
> - If the container restarts with existing data, scripts will NOT run again. This is desired behavior.

---

## Running the Project

## Local Setup

### 1. Create Database

For linux users, run these commands in the terminal:

```bash
# switch to postgres system user
sudo -iu postgres

# open PostgreSQL interactive shell
psql

# list all databases
\l

# create a new database
CREATE DATABASE discordv2;

# exit psql
\q

# back to normal shell
exit
```

---

### 2. Apply Schema

```bash
psql -U postgres -d discordv2 -f db/schema.sql
```

---

### 3. Set Environment Variables

```bash
export PSQL_URI="postgres://postgres:password@127.0.0.1:5432/discordv2"
export RDB_URI="127.0.0.1:6379"
export RDB_PASSWORD=""
```

About PSQL_URI, check out the [docs](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING).

---

### 4. Build and Run

```bash
cd app/

go build -o chat-app cmd/server/main.go

./chat-app
```

---

## Running on Minikube

### 1. Deploy PostgreSQL and Redis

```bash
kubectl apply -f postgres.yaml
kubectl apply -f redis-store.yaml
```

Verify:

```bash
kubectl get all
```

Ensure that `redis-store` and `postgres` are listed and their `STATUS` is `Running`.

---

### 2. Deploy Application

```bash
kubectl apply -f components/
```

Verify:

```bash
kubectl get all
```

Ensure that:
- `chat-ms` pods are listed
- Their `STATUS` is `Running`
- A corresponding `chat-ms` service exists

---

### 3. Expose Service (Minikube Tunnel)

```bash
minikube tunnel
```

In a new terminal:

```bash
kubectl get svc chat-service
```

Verify:
- Look for the `EXTERNAL-IP` field
- Access the app at:

```bash
http://<External-IP>:8080
```

Or test with:

```bash
curl http://<External-IP>:8080
```

Notes:
- `minikube tunnel` must keep running in the background
- Required for `LoadBalancer` services on Minikube

> [!IMPORTANT]
> Update the URI in `tests/main.py` for testing to `<External-IP>:8080`

More info about **services** can be found in the [docs](https://kubernetes.io/docs/concepts/services-networking/service/).

---

### 4. Adjust Replicas (Optional)

Edit file:
`components/chat-ms.yaml`

Change:
```yaml
# kubernetes will adjust number of running pods (instances of that deployment)
spec.replicas: 1
```

More info about **deployments** can be found in the [docs](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/).

---

### 5. View Logs

This is useful for debugging.

```bash
kubectl get pods

kubectl logs -f pod/<pod-id>
```

Furthermore, run
```bash
kubectl describe pod/<pod-id>
```
to get a detailed description of the Pod.

More info about **Pods** can be found in the [docs](https://kubernetes.io/docs/concepts/workloads/pods/).

---

## Testing

You **must** have `python` installed to run the test script provided in `tests` directory.

Installation: https://www.python.org/downloads/

```bash
python tests/main.py
```

---

## API Endpoints

Base URL:
`http://127.0.0.1:8080`

OR (for minikube): `http://<External-IP>:8080`

| Method | Endpoint                      | Description                     |
|--------|------------------------------|---------------------------------|
| POST   | /api/register                | Register a new user            |
| POST   | /api/{guild_id}              | Join a guild                   |
| POST   | /api/{guild_id}/{channel_id} | Send a message                 |
| GET    | /ws/                         | Initialize WebSocket           |
| GET    | /ws/{guild_id}/{channel_id}  | Switch guild/channel           |

---
# Kafka consumer group operator

kconsumer-group-operator is a kubernetes operator built to manage Kafka consumer group. The details of this design is
described in an accompanying blog at thenextapps.com/2020/06/19/managing-kafka-consumers-with-operator-pattern/

## Installation

Prerequisite:
- Kubernetes v1.16.5 or later
- kubectl installed and configured for the cluster
- go version go1.14.3
- operator-sdk `brew install operator-sdk`
- helm `brew install helm`

Install `kube-prometheus` at `https://github.com/coreos/kube-prometheus#quickstart`

```sh
git clone https://github.com/coreos/kube-prometheus
cd kube-prometheus
git checkout release-0.4
kubectl create -f manifests/setup
until kubectl get servicemonitors --all-namespaces ; do date; sleep 1; echo ""; done
kubectl create -f manifests/
# wait for pods are up and running in monitoring namespace
# disable default prometheus adapter
kubectl scale rc prometheus-adapter --replicas=0 -n monitoring
# access dashboards via localhost
kubectl --namespace monitoring port-forward svc/prometheus-k8s 9090
kubectl --namespace monitoring port-forward svc/grafana 3000
kubectl --namespace monitoring port-forward svc/alertmanager-main 9093
```

Install `prometheus-adapter` with helm

```sh
helm upgrade --install  --wait -f ./prometheus-adapter-config/prom.yaml prom-adapter stable/prometheus-adapter -n monitoring
```

Install Strimzi operator for kafka

```sh
kubectl apply -f 'https://strimzi.io/install/latest?namespace=default' -n default
kubectl apply -f https://strimzi.io/examples/latest/kafka/kafka-persistent-single.yaml -n default
kubectl wait kafka/my-cluster --for=condition=Ready --timeout=300s -n default
```

Pre-built images are available at https://hub.docker.com/u/thenextapps/ but you can rebuild with following step:

Run unit test

```sh
go test -timeout 30s github.com/thanhnamit/kconsumer-group-operator/pkg/controller/kconsumergroup -run TestKconsumerGroupController
```

Run e2e test

```sh
operator-sdk test local ./test/e2e --operator-namespace default
```

Build image

```sh
operator-sdk build thenextapps/kconsumer-group-operator:v0.0.1
# optionally push to registry
docker push thenextapps/kconsumer-group-operator:v0.0.1
```

## Usage

Create Kafka topic and run producer

```sh
kubectl create -f apps/kproducer/k8s/create-kafka-topic.yaml
kubectl wait KafkaTopic/fast-data-topic --for=condition=Ready --timeout=300s
kubectl create -f apps/kproducer/k8s/kproducer-deployment.yaml
```

Deploy manifests for operator

```sh
kubectl create -f deploy/crds/thenextapps.com_kconsumergroups_crd.yaml
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
kubectl create -f deploy/operator.yaml
```

Deploy primary resource (managed by the operator)

```sh
kubectl create -f deploy/crds/thenextapps.com_v1alpha1_kconsumergroup_cr.yaml
kubectl get KconsumerGroup kconsumer -o yaml
```

Verify all child resources are created

```sh
kubectl get HorizontalPodAutoscaler kconsumer -o yaml
kubectl get ServiceMonitor kconsumer -o yaml
kubectl get PrometheusRule kconsumer -o yaml
kubectl get service kconsumer -o yaml
kubectl get deployment kconsumer -o yaml
```

The final state of the default namespace should look like:

```sh
kubectl get pods
NAME                                          READY   STATUS    RESTARTS   AGE
kconsumer-6c5f87c746-ptsbg                    1/1     Running   0          20h
kconsumer-group-operator-7db8f9b8c8-whh5p     1/1     Running   0          158m
kproducer-58b7bb7d66-bzlwg                    1/1     Running   0          26h
my-cluster-entity-operator-68b7df59b9-bvtx2   3/3     Running   2          26h
my-cluster-kafka-0                            2/2     Running   0          26h
my-cluster-zookeeper-0                        1/1     Running   0          26h
strimzi-cluster-operator-6c9d899778-fhdlt     1/1     Running   1          26h
```

Check metric scraping status at `http://localhost:9090/targets` 

Send message via producer's api

```sh
curl http://localhost:8083/send/10000
```

Watch HPA in action

```sh
+ kubectl get hpa -w
NAME        REFERENCE              TARGETS   MINPODS   MAXPODS   REPLICAS   AGE
kconsumer   Deployment/kconsumer   0/1k      1         3         1          53s
kconsumer   Deployment/kconsumer   3412/1k   1         3         1          107s
kconsumer   Deployment/kconsumer   3412/1k   1         3         3          2m1s
kconsumer   Deployment/kconsumer   2198333m/1k   1         3         3          2m17s
kconsumer   Deployment/kconsumer   1061/1k       1         3         3          2m32s
kconsumer   Deployment/kconsumer   0/1k          1         3         3          3m18s
kconsumer   Deployment/kconsumer   0/1k          1         3         3          7m24s
kconsumer   Deployment/kconsumer   0/1k          1         3         3          8m10s
kconsumer   Deployment/kconsumer   0/1k          1         3         1          8m25s
```

## License
[MIT](https://choosealicense.com/licenses/mit/)
MINIKUBE_PROFILE := omdemo
OPEN_MATCH_VERSION := 1.2.0

dev:
	skaffold dev --minikube-profile $(MINIKUBE_PROFILE) --port-forward --tail

up:
	minikube start -p $(MINIKUBE_PROFILE) --cpus=3 --memory=2500mb
	minikube profile $(MINIKUBE_PROFILE)
	helm repo add open-match https://open-match.dev/chart/stable
	helm install open-match --namespace open-match --create-namespace open-match/open-match \
	  --version=v$(OPEN_MATCH_VERSION) \
	  --set open-match-customize.enabled=true \
	  --set open-match-customize.evaluator.enabled=true \
	  --set open-match-override.enabled=true \
	  --set frontend.replicas=1 \
	  --set backend.replicas=1 \
	  --set query.replicas=1 \
	  --set function.replicas=1 \
	  --set swaggerui.replicas=1 \
	  --set redis.cluster.slaveCount=1

down:
	minikube stop -p $(MINIKUBE_PROFILE)

delete:
	minikube delete -p $(MINIKUBE_PROFILE)

monitor-redis:
	kubectl exec -n open-match open-match-redis-node-0 -- redis-cli monitor | grep -v 'ping\|PING\|PUBLISH\|INFO'

log-matchfunction:
	kubectl logs -f -n default matchfunction

test:
	cd matchfunction/ && go test -count=1 ./...
	cd tests/ && go test -count=1 ./...

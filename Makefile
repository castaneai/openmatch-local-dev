MINIKUBE_PROFILE := omdemo

dev:
	skaffold dev --minikube-profile $(MINIKUBE_PROFILE) --port-forward --tail

up:
	minikube start -p $(MINIKUBE_PROFILE) --cpus=3 --memory=2500mb --kubernetes-version=v1.21.14
	helmfile sync

down:
	minikube stop -p $(MINIKUBE_PROFILE)

delete:
	minikube delete -p $(MINIKUBE_PROFILE)

monitor-redis:
	kubectl exec -n open-match open-match-redis-master-0 -- redis-cli monitor | grep -v 'ping\|PING\|PUBLISH\|INFO'

clear-redis:
	kubectl exec -n open-match open-match-redis-master-0 -- redis-cli flushall

log-matchfunction:
	kubectl logs -f -n default matchfunction

test:
	go test -count=1 ./...

MINIKUBE_PROFILE := omdemo

dev:
	skaffold dev --minikube-profile $(MINIKUBE_PROFILE) --port-forward --tail

up:
	minikube start -p $(MINIKUBE_PROFILE) --cpus=3 --memory=2500mb
	helmfile sync

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

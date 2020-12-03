MINIKUBE_PROFILE := omdemo
OPEN_MATCH_VERSION := 1.0.0

dev:
	skaffold dev --minikube-profile $(MINIKUBE_PROFILE) --port-forward --tail

up-minikube:
	minikube start -p $(MINIKUBE_PROFILE) --cpus=3 --memory=2500mb
	minikube profile $(MINIKUBE_PROFILE)

up-openmatch:
	helm repo add open-match https://open-match.dev/chart/stable
	helm install openmatch --namespace open-match --create-namespace open-match/open-match \
	  --version=v$(OPEN_MATCH_VERSION) \
	  --set open-match-customize.enabled=true \
	  --set open-match-customize.evaluator.enabled=true \
	  --set open-match-override.enabled=true

monitor-redis:
	kubectl exec -n open-match om-redis-master-0 -- redis-cli monitor | grep -v 'ping\|PING\|PUBLISH\|INFO'

clear-redis:
	kubectl exec -n open-match om-redis-master-0 -- redis-cli FLUSHALL

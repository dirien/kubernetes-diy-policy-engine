export KO_DOCKER_REPO := ghcr.io/dirien

.PHONY: ko
ko:
	ko build .  -B

.PHONY: deploy-webhook
deploy-webhook:
	kubectl delete -f deploy/validating-webhook-configuration.yaml --ignore-not-found
	kubectl apply -k deploy/
	kubectl wait --for=condition=available --timeout=60s deployment/k8s-diy-validating-webhook
	kubectl apply -f deploy/validating-webhook-configuration.yaml

.PHONY: delete-webhook
delete-webhook:
	kubectl delete -f deploy/validating-webhook-configuration.yaml --ignore-not-found
	kubectl delete -k deploy/ --ignore-not-found

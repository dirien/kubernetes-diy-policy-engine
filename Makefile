.PHONY: cert-manager
cert-manager:
	helm repo add cert-manager https://charts.jetstack.io
	helm repo update
	helm upgrade -i cert-manger cert-manager/cert-manager --version 1.9.0 --set installCRDs=true

.PHONY: deploy-certs
deploy-certs:
	kubectl apply -k deploy/

.PHONY: make-images
make-images:
	cd k8s-diy-mutating-webhook && make ko && cd ..
	cd k8s-diy-validating-webhook && make ko && cd ..

.PHONY: deploy-webhooks
deploy-webhooks:
	cd k8s-diy-mutating-webhook && make deploy-webhook && cd ..
	cd k8s-diy-validating-webhook && make deploy-webhook && cd ..

.PHONY: all
all: cert-manager deploy-certs make-images deploy-webhooks



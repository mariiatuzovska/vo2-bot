.PHONY: tf-init tf-up tf-down dev clear

tf-init:
	cd infra && terraform init

tf-up:
	cd infra && terraform apply -auto-approve

tf-down:
	cd infra && terraform destroy -auto-approve

dev:
	tilt up

clear:
	tilt down
	$(MAKE) tf-down

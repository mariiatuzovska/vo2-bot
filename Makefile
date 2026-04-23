.PHONY: init tf-init tf-up tf-down dev clear sqlc-install sqlc

init: .env infra/terraform.tfvars tf-init sqlc-install
	@echo "init: done. Fill in .env + infra/terraform.tfvars, then run 'make dev'."

.env:
	cp .env.example .env
	@echo "init: created .env from template."

infra/terraform.tfvars:
	cp infra/terraform.tfvars.example infra/terraform.tfvars
	@echo "init: created infra/terraform.tfvars from template."

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

sqlc-install:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1

sqlc:
	cd db && $$(go env GOPATH)/bin/sqlc generate

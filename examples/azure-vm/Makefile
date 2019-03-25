
.PHONY: help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-10s\033[0m %s\n", $$1, $$2}'

clean: ## Remove files created by terraform
	rm -rf terraform.tfstate* plan.out

deploy: ## Deploy project to remote cloud or VM
	terraform init
	terraform plan -out=plan.out
	terraform apply plan.out

destroy: ## Destroy project and associated resources
	terraform destroy

login: ## SSH/Login to the VM
	ssh $(terraform output admin_username)@$(terraform output public_ip_address)

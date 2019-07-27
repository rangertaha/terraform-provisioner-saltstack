# Terraform SaltStack Provisioner 


<img src="https://cdn.rawgit.com/hashicorp/terraform-website/master/content/source/assets/images/logo-hashicorp.svg" width="600px">

## Dependencies

* [Golang](https://golang.org)

## Build


```bash
make build
```
 
 
 Copy the `.terraform` to your terraform project 


## Example

### Build 

Build plugin

```bash
make build
```


### Login

Login to Azure generating data required for the environment variables. 

```bash
az login
```

### Variables

Set environment variables with from following

```bash
cat << EOF >> ~/.profile
export TF_VAR_subscription_id=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export TF_VAR_client_id=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export TF_VAR_client_secret=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export TF_VAR_tenant_id=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
EOF
```

### Execute

Execute Terraform to deploy the infrastructure. 

```bash
cd examples/azure-vm
```

```bash
make deploy 
```

## Maintainers


rangertaha



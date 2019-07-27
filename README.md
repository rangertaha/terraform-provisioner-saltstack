# Terraform SaltStack Provisioner 

<img src="https://travis-ci.org/rangertaha/terraform-provisioner-saltstack.svg?branch=master" width="600px">


This is an alternative SaltStack provisioner for Terraform

<img src="https://cdn.rawgit.com/hashicorp/terraform-website/master/content/source/assets/images/logo-hashicorp.svg" width="600px">

## Dependencies

* [Golang](https://golang.org)

## Build


```bash
make build
```
 
 
 Copy the `.terraform` to your terraform project 


### Example

#### Setup

Building the plugin also places a copy of  `.terraform` in the example project

```bash
make build
```


The Azure example requires us to run the az tool to login and view credentials 
required for the environment variables. 

```bash
az login
```

Set Terraform environment variables for the Azure provider

```bash
cat << EOF >> ~/.profile
export TF_VAR_subscription_id=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export TF_VAR_client_id=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export TF_VAR_client_secret=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export TF_VAR_tenant_id=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
EOF
```

#### Execute

Execute Terraform to deploy the infrastructure. 

```bash
cd examples/azure-vm
```

```bash
make deploy 
```

## Maintainers


rangertaha



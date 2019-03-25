# Example Azure VM


Named after the Roman goddess of wisdom and strategic warfare and the sponsor of arts, trade, and strategy. 
This project is used to collect data from public sources for use in FTR trading. 


### Environment Variables

In Linux/Mac OSX you can do the following and update the 'xxxx' with the 
actual values. Do the equivalent for Windows OS

```bash
cat << EOF >> ~/.bash_profile
export TF_VAR_subscription_id=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export TF_VAR_client_id=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export TF_VAR_client_secret=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export TF_VAR_tenant_id=xxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
EOF
```

### Execute

Execute terraform to create the VM in azure and install the project.

```bash
terraform init
terraform plan -out=plan.out
terraform apply plan.out

```

Login with `ssh rangertaha@XXX.XXX.XXX.XXX`...




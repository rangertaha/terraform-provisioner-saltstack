package main

import (
	"github.com/hashicorp/terraform/plugin"
	"github.com/rangertaha/terraform-provisioner-saltstack/saltstack"
	)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProvisionerFunc: saltstack.Provisioner,
	})
}

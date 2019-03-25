/* Used to install on VM directly
resource "null_resource" "vm" {

  connection {
    host = "XX.XX.XX.XX"
  }

  provisioner "salt-masterless" {
    local_state_tree = "salt"
    remote_state_tree = "/srv/salt"

    connection {
      type = "ssh"
      user = "root"
      password = "toor"
    }
  }
}
*/

provider "azurerm" {
}

//data "local_file" "ssh-pub-key" {
//  filename = "/Users/betm/.ssh/id_rsa.pub"
//}

//data "local_file" "ssh-pri-key" {
//  filename = "/Users/betm/.ssh/id_rsa"
//}



# Create a resource group if it doesn’t exist
resource "azurerm_resource_group" "exampleterraformgroup" {
  name = "ExampleResourceGroup"
  location = "eastus"

  tags {
    environment = "dev"
  }
}

# Create virtual network
resource "azurerm_virtual_network" "exampleterraformnetwork" {
  name = "ExampleVnet"
  address_space = [
    "10.0.0.0/16"]
  location = "eastus"
  resource_group_name = "${azurerm_resource_group.exampleterraformgroup.name}"

  tags {
    environment = "dev"
  }
}

# Create subnet
resource "azurerm_subnet" "exampleterraformsubnet" {
  name = "ExampleSubnet"
  resource_group_name = "${azurerm_resource_group.exampleterraformgroup.name}"
  virtual_network_name = "${azurerm_virtual_network.exampleterraformnetwork.name}"
  address_prefix = "10.0.1.0/24"
}

# Create public IPs
resource "azurerm_public_ip" "exampleterraformpublicip" {
  name = "ExamplePublicIP"
  location = "eastus"
  resource_group_name = "${azurerm_resource_group.exampleterraformgroup.name}"
  allocation_method = "Dynamic"
  idle_timeout_in_minutes = 30

  tags {
    environment = "dev"
  }
}

# Create Network Security Group and rule
resource "azurerm_network_security_group" "exampleterraformnsg" {
  name = "ExampleNetworkSecurityGroup"
  location = "eastus"
  resource_group_name = "${azurerm_resource_group.exampleterraformgroup.name}"

  //  security_rule {
  //    name = "SSH"
  //    priority = 1001
  //    direction = "Inbound"
  //    access = "Allow"
  //    protocol = "Tcp"
  //    source_port_range = "*"
  //    destination_port_range = "21-9200"
  //    source_address_prefix = "158.81.192.22"
  //    destination_address_prefix = "*"
  //  }
  security_rule {
    name = "SSH"
    priority = 1001
    direction = "Inbound"
    access = "Allow"
    protocol = "Tcp"
    source_port_range = "*"
    destination_port_range = "*"
    source_address_prefix = "*"
    destination_address_prefix = "*"
  }

  tags {
    environment = "dev"
  }
}

# Create network interface
resource "azurerm_network_interface" "exampleterraformnic" {
  name = "ExampleNIC"
  location = "eastus"
  resource_group_name = "${azurerm_resource_group.exampleterraformgroup.name}"
  network_security_group_id = "${azurerm_network_security_group.exampleterraformnsg.id}"

  ip_configuration {
    name = "exampleNicConfiguration"
    subnet_id = "${azurerm_subnet.exampleterraformsubnet.id}"
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id = "${azurerm_public_ip.exampleterraformpublicip.id}"
  }

  tags {
    environment = "dev"
  }
}

# Generate random text for a unique storage account name
resource "random_id" "exampleId" {
  keepers = {
    # Generate a new ID only when a new resource group is defined
    resource_group = "${azurerm_resource_group.exampleterraformgroup.name}"
  }

  byte_length = 8
}

# Create storage account for boot diagnostics
resource "azurerm_storage_account" "examplestorageaccount" {
  name = "diag${random_id.exampleId.hex}"
  resource_group_name = "${azurerm_resource_group.exampleterraformgroup.name}"
  location = "eastus"
  account_tier = "Standard"
  account_replication_type = "LRS"

  tags {
    environment = "dev"
  }
}

//resource "azurerm_managed_disk" "examplevmosdisk" {
//  name = "ExampleOsDisk"
//  resource_group_name = "${azurerm_resource_group.exampleterraformgroup.name}"
//}
//

# Create virtual machine
resource "azurerm_virtual_machine" "examplevm" {
  name = "ExampleVM"
  location = "eastus"
  resource_group_name = "${azurerm_resource_group.exampleterraformgroup.name}"
  network_interface_ids = [
    "${azurerm_network_interface.exampleterraformnic.id}"]
  vm_size = "Standard_DS1_v2"

  delete_os_disk_on_termination = true
  storage_os_disk {
    name = "ExampleOsDisk"
    caching = "ReadWrite"
    create_option = "FromImage"
    managed_disk_type = "Premium_LRS"
    disk_size_gb = "60"
  }

  storage_image_reference {
    publisher = "OpenLogic"
    offer = "CentOS"
    sku = "7.3"
    version = "latest"
  }

  os_profile {
    computer_name = "example"
    admin_username = "${var.admin_username}"
    admin_password = "${var.admin_password}"
  }

  os_profile_linux_config {
    disable_password_authentication = false
    ssh_keys {
      path = "/home/${var.admin_username}/.ssh/authorized_keys"
      key_data = "${file("~/.ssh/id_rsa.pub")}"
    }
  }

  boot_diagnostics {
    enabled = "true"
    storage_uri = "${azurerm_storage_account.examplestorageaccount.primary_blob_endpoint}"
  }

  tags {
    environment = "dev"
  }




  provisioner "saltstack" {
    //enabled = true
    //refresh = true
    //remove = false
    //grains = true
    //tfvars = "terraform.tfvars"
    # local_pillar_roots = "${var.local_pillar_roots}"
    # remote_pillar_roots = "${var.remote_pillar_roots}"
    local_state_tree = "../../salt/states"
    remote_state_tree = "/srv/salt"
    sudo_password = "${var.admin_password}"
    connection {
      type = "ssh"
      user = "${var.admin_username}"
      password = "${var.admin_password}"
    }
  }



}

data "azurerm_public_ip" "example" {
  name = "${azurerm_public_ip.exampleterraformpublicip.name}"
  resource_group_name = "${azurerm_virtual_machine.examplevm.resource_group_name}"
}

//output "admin_username" {
//  value = "${var.admin_username}"
//}
//
//output "public_ip_address" {
//  value = "${data.azurerm_public_ip.example.ip_address}"
//}

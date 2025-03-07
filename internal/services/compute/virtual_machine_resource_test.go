package compute_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/terraform-provider-azurestack/internal/clients"
	"github.com/hashicorp/terraform-provider-azurestack/internal/services/compute/parse"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurestack/internal/utils"
	"github.com/tombuildsstuff/giovanni/storage/2018-11-09/blob/blobs"
)

type VirtualMachineResource struct{}

func TestAccVirtualMachine_winTimeZone(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_virtual_machine", "test")
	r := VirtualMachineResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.winTimeZone(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
	})
}

func (VirtualMachineResource) Exists(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := parse.VirtualMachineID(state.ID)
	if err != nil {
		return nil, err
	}

	resp, err := clients.Compute.VMClient.Get(ctx, id.ResourceGroup, id.Name, "")
	if err != nil {
		return nil, fmt.Errorf("retrieving Compute Virtual Machine %q", id)
	}

	return pointer.FromBool(resp.ID != nil), nil
}

func (VirtualMachineResource) managedDiskExists(diskId *string, shouldExist bool) acceptance.ClientCheckFunc {
	return func(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) error {
		id, err := parse.ManagedDiskID(*diskId)
		if err != nil {
			return err
		}

		disk, err := clients.Compute.DisksClient.Get(ctx, id.ResourceGroup, id.DiskName)
		if err != nil {
			if utils.ResponseWasNotFound(disk.Response) {
				if !shouldExist {
					return nil
				}

				return fmt.Errorf("disk %s does not exist", *id)
			}
			return err
		}

		if !shouldExist {
			return fmt.Errorf("disk %s shouldn't exist but it does", *id)
		}

		return nil
	}
}

func (VirtualMachineResource) findManagedDiskID(field string, managedDiskID *string) acceptance.ClientCheckFunc {
	return func(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) error {
		id, err := parse.VirtualMachineID(state.ID)
		if err != nil {
			return err
		}

		virtualMachine, err := clients.Compute.VMClient.Get(ctx, id.ResourceGroup, id.Name, "")
		if err != nil {
			return err
		}
		if virtualMachine.VirtualMachineProperties == nil {
			return fmt.Errorf("`properties` was nil")
		}
		if virtualMachine.VirtualMachineProperties.StorageProfile == nil {
			return fmt.Errorf("`properties.StorageProfile` was nil")
		}

		diskName := state.Attributes[field]

		if osDisk := virtualMachine.VirtualMachineProperties.StorageProfile.OsDisk; osDisk != nil {
			if osDisk.Name != nil && osDisk.ManagedDisk != nil && osDisk.ManagedDisk.ID != nil {
				if *osDisk.Name == diskName {
					*managedDiskID = *osDisk.ManagedDisk.ID
					return nil
				}
			}
		}

		if dataDisks := virtualMachine.VirtualMachineProperties.StorageProfile.DataDisks; dataDisks != nil {
			for _, dataDisk := range *dataDisks {
				if dataDisk.Name == nil || dataDisk.ManagedDisk == nil || dataDisk.ManagedDisk.ID == nil {
					continue
				}

				if *dataDisk.Name == diskName {
					*managedDiskID = *dataDisk.ManagedDisk.ID
					return nil
				}
			}
		}

		return fmt.Errorf("unable to locate disk %q", diskName)
	}
}

func (VirtualMachineResource) deallocate(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) error {
	vmID, err := parse.VirtualMachineID(state.ID)
	if err != nil {
		return err
	}

	name := vmID.Name
	resourceGroup := vmID.ResourceGroup

	future, err := client.Compute.VMClient.Deallocate(ctx, resourceGroup, name)
	if err != nil {
		return fmt.Errorf("Failed stopping virtual machine %q: %+v", resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Compute.VMClient.Client); err != nil {
		return fmt.Errorf("Failed long polling for the stop of virtual machine %q: %+v", resourceGroup, err)
	}

	return nil
}

func (VirtualMachineResource) unmanagedDiskExistsInContainer(blobName string, shouldExist bool) acceptance.ClientCheckFunc {
	return func(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) error {
		accountName := state.Attributes["storage_account_name"]
		containerName := state.Attributes["name"]

		account, err := clients.Storage.FindAccount(ctx, accountName)
		if err != nil {
			return fmt.Errorf("retrieving Account %q for Blob %q (Container %q): %s", accountName, blobName, containerName, err)
		}
		if account == nil {
			return fmt.Errorf("Unable to locate Storage Account %q!", accountName)
		}

		client, err := clients.Storage.BlobsClient(ctx, *account)
		if err != nil {
			return fmt.Errorf("building Blobs Client: %s", err)
		}

		input := blobs.GetPropertiesInput{}
		props, err := client.GetProperties(ctx, accountName, containerName, blobName, input)
		if err != nil {
			if utils.ResponseWasNotFound(props.Response) {
				if !shouldExist {
					return nil
				}

				return fmt.Errorf("The Blob for the Unmanaged Disk %q should exist in the Container %q but it didn't!", blobName, containerName)
			}

			return fmt.Errorf("retrieving properties for Blob %q (Container %q): %s", blobName, containerName, err)
		}

		if !shouldExist {
			return fmt.Errorf("The Blob for the Unmanaged Disk %q shouldn't exist in the Container %q but it did!", blobName, containerName)
		}

		return nil
	}
}

func (VirtualMachineResource) Destroy(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	vmName := state.Attributes["name"]
	resourceGroup := state.Attributes["resource_group_name"]

	var forceDelete *bool = nil
	future, err := client.Compute.VMClient.Delete(ctx, resourceGroup, vmName, forceDelete)
	if err != nil {
		return nil, fmt.Errorf("Bad: Delete on vmClient: %+v", err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Compute.VMClient.Client); err != nil {
		return nil, fmt.Errorf("Bad: Delete on vmClient: %+v", err)
	}

	return pointer.FromBool(true), nil
}

func (VirtualMachineResource) winTimeZone(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_virtual_network" "test" {
  name                = "acctvn-%d"
  address_space       = ["10.0.0.0/16"]
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_subnet" "test" {
  name                 = "acctsub-%d"
  resource_group_name  = azurestack_resource_group.test.name
  virtual_network_name = azurestack_virtual_network.test.name
  address_prefix       = "10.0.2.0/24"
}

resource "azurestack_network_interface" "test" {
  name                = "acctni-%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  ip_configuration {
    name                          = "testconfiguration1"
    subnet_id                     = azurestack_subnet.test.id
    private_ip_address_allocation = "Dynamic"
  }
}

resource "azurestack_storage_account" "test" {
  name                     = "accsa%d"
  resource_group_name      = azurestack_resource_group.test.name
  location                 = azurestack_resource_group.test.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
}

resource "azurestack_storage_container" "test" {
  name                  = "vhds"
  storage_account_name  = azurestack_storage_account.test.name
  container_access_type = "private"
}

resource "azurestack_virtual_machine" "test" {
  name                  = "acctvm-%d"
  location              = azurestack_resource_group.test.location
  resource_group_name   = azurestack_resource_group.test.name
  network_interface_ids = [azurestack_network_interface.test.id]
  vm_size               = "Standard_D1_v2"

  storage_image_reference {
    publisher = "MicrosoftWindowsServer"
    offer     = "WindowsServer"
    sku       = "2012-Datacenter-smalldisk"
    version   = "latest"
  }

  storage_os_disk {
    name          = "myosdisk1"
    vhd_uri       = "${azurestack_storage_account.test.primary_blob_endpoint}${azurestack_storage_container.test.name}/myosdisk1.vhd"
    caching       = "ReadWrite"
    create_option = "FromImage"
  }

  os_profile {
    computer_name  = "winhost01"
    admin_username = "testadmin"
    admin_password = "Password1234!"
  }

  os_profile_windows_config {
    timezone = "Pacific Standard Time"
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger)
}

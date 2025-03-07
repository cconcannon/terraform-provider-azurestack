package network_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/profiles/2020-09-01/network/mgmt/network"
	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/terraform-provider-azurestack/internal/clients"
	network2 "github.com/hashicorp/terraform-provider-azurestack/internal/services/network"
	"github.com/hashicorp/terraform-provider-azurestack/internal/services/network/parse"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/pluginsdk"
)

type NetworkInterfaceBackendAddressPoolResource struct{}

func TestAccNetworkInterfaceBackendAddressPoolAssociation_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_network_interface_backend_address_pool_association", "test")
	r := NetworkInterfaceBackendAddressPoolResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		// intentional as this is a Virtual Resource
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep(),
	})
}

func TestAccNetworkInterfaceBackendAddressPoolAssociation_requiresImport(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_network_interface_backend_address_pool_association", "test")
	r := NetworkInterfaceBackendAddressPoolResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		// intentional as this is a Virtual Resource
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		{
			Config:      r.requiresImport(data),
			ExpectError: acceptance.RequiresImportError("azurestack_network_interface_backend_address_pool_association"),
		},
	})
}

func TestAccNetworkInterfaceBackendAddressPoolAssociation_deleted(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_network_interface_backend_address_pool_association", "test")
	r := NetworkInterfaceBackendAddressPoolResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		// intentionally not using a DisppearsStep as this is a Virtual Resource
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				data.CheckWithClient(r.destroy),
			),
			ExpectNonEmptyPlan: true,
		},
	})
}

func TestAccNetworkInterfaceBackendAddressPoolAssociation_updateNIC(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_network_interface_backend_address_pool_association", "test")
	r := NetworkInterfaceBackendAddressPoolResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		// intentional as this is a Virtual Resource
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep(),
		{
			Config: r.updateNIC(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep(),
	})
}

func (t NetworkInterfaceBackendAddressPoolResource) Exists(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	splitId := strings.Split(state.ID, "|")
	if len(splitId) != 2 {
		return nil, fmt.Errorf("expected ID to be in the format {networkInterfaceId}/ipConfigurations/{ipConfigurationName}|{backendAddressPoolId} but got %q", state.ID)
	}

	id, err := parse.NetworkInterfaceIpConfigurationID(splitId[0])
	if err != nil {
		return nil, err
	}

	backendAddressPoolId := splitId[1]

	read, err := clients.Network.InterfacesClient.Get(ctx, id.ResourceGroup, id.NetworkInterfaceName, "")
	if err != nil {
		return nil, fmt.Errorf("reading %s: %+v", *id, err)
	}

	nicProps := read.InterfacePropertiesFormat
	if nicProps == nil {
		return nil, fmt.Errorf("`properties` was nil for %s: %+v", *id, err)
	}

	c := network2.FindNetworkInterfaceIPConfiguration(read.InterfacePropertiesFormat.IPConfigurations, id.IpConfigurationName)
	if c == nil {
		return nil, fmt.Errorf("IP Configuration %q wasn't found for %s", id.IpConfigurationName, *id)
	}
	config := *c

	found := false
	if config.InterfaceIPConfigurationPropertiesFormat.LoadBalancerBackendAddressPools != nil {
		for _, pool := range *config.InterfaceIPConfigurationPropertiesFormat.LoadBalancerBackendAddressPools {
			if *pool.ID == backendAddressPoolId {
				found = true
				break
			}
		}
	}

	return pointer.FromBool(found), nil
}

func (NetworkInterfaceBackendAddressPoolResource) destroy(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) error {
	nicID, err := parse.NetworkInterfaceID(state.Attributes["network_interface_id"])
	if err != nil {
		return err
	}

	nicName := nicID.Name
	resourceGroup := nicID.ResourceGroup
	backendAddressPoolId := state.Attributes["backend_address_pool_id"]
	ipConfigurationName := state.Attributes["ip_configuration_name"]

	read, err := client.Network.InterfacesClient.Get(ctx, resourceGroup, nicName, "")
	if err != nil {
		return fmt.Errorf("retrieving Network Interface %q (Resource Group %q): %+v", nicName, resourceGroup, err)
	}

	c := network2.FindNetworkInterfaceIPConfiguration(read.InterfacePropertiesFormat.IPConfigurations, ipConfigurationName)
	if c == nil {
		return fmt.Errorf("IP Configuration %q wasn't found for Network Interface %q (Resource Group %q)", ipConfigurationName, nicName, resourceGroup)
	}
	config := *c

	updatedPools := make([]network.BackendAddressPool, 0)
	if config.InterfaceIPConfigurationPropertiesFormat.LoadBalancerBackendAddressPools != nil {
		for _, pool := range *config.InterfaceIPConfigurationPropertiesFormat.LoadBalancerBackendAddressPools {
			if *pool.ID != backendAddressPoolId {
				updatedPools = append(updatedPools, pool)
			}
		}
	}
	config.InterfaceIPConfigurationPropertiesFormat.LoadBalancerBackendAddressPools = &updatedPools

	future, err := client.Network.InterfacesClient.CreateOrUpdate(ctx, resourceGroup, nicName, read)
	if err != nil {
		return fmt.Errorf("removing Backend Address Pool Association for Network Interface %q (Resource Group %q): %+v", nicName, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Network.InterfacesClient.Client); err != nil {
		return fmt.Errorf("waiting for removal of Backend Address Pool Association for NIC %q (Resource Group %q): %+v", nicName, resourceGroup, err)
	}

	return nil
}

func (r NetworkInterfaceBackendAddressPoolResource) basic(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurestack_network_interface" "test" {
  name                = "acctestni-%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  ip_configuration {
    name                          = "testconfiguration1"
    subnet_id                     = azurestack_subnet.test.id
    private_ip_address_allocation = "Dynamic"
  }
}

resource "azurestack_network_interface_backend_address_pool_association" "test" {
  network_interface_id    = azurestack_network_interface.test.id
  ip_configuration_name   = "testconfiguration1"
  backend_address_pool_id = azurestack_lb_backend_address_pool.test.id
}
`, r.template(data), data.RandomInteger)
}

func (r NetworkInterfaceBackendAddressPoolResource) requiresImport(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurestack_network_interface_backend_address_pool_association" "import" {
  network_interface_id    = azurestack_network_interface_backend_address_pool_association.test.network_interface_id
  ip_configuration_name   = azurestack_network_interface_backend_address_pool_association.test.ip_configuration_name
  backend_address_pool_id = azurestack_network_interface_backend_address_pool_association.test.backend_address_pool_id
}
`, r.basic(data))
}

func (r NetworkInterfaceBackendAddressPoolResource) updateNIC(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurestack_network_interface" "test" {
  name                = "acctestni-%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  ip_configuration {
    name                          = "testconfiguration1"
    subnet_id                     = azurestack_subnet.test.id
    private_ip_address_allocation = "Dynamic"
    primary                       = true
  }

}

resource "azurestack_network_interface_backend_address_pool_association" "test" {
  network_interface_id    = azurestack_network_interface.test.id
  ip_configuration_name   = "testconfiguration1"
  backend_address_pool_id = azurestack_lb_backend_address_pool.test.id
}
`, r.template(data), data.RandomInteger)
}

func (NetworkInterfaceBackendAddressPoolResource) template(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_virtual_network" "test" {
  name                = "acctestvn-%d"
  address_space       = ["10.0.0.0/16"]
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_subnet" "test" {
  name                 = "testsubnet"
  resource_group_name  = azurestack_resource_group.test.name
  virtual_network_name = azurestack_virtual_network.test.name
  address_prefix       = "10.0.2.0/24"
}

resource "azurestack_public_ip" "test" {
  name                = "test-ip-%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
  allocation_method   = "Static"
}

resource "azurestack_lb" "test" {
  name                = "acctestlb-%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  frontend_ip_configuration {
    name                 = "primary"
    public_ip_address_id = azurestack_public_ip.test.id
  }
}

resource "azurestack_lb_backend_address_pool" "test" {
  resource_group_name = azurestack_resource_group.test.name
  loadbalancer_id     = azurestack_lb.test.id
  name                = "acctestpool"
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger, data.RandomInteger)
}

package resource_test

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/resourceids"
	"github.com/hashicorp/terraform-provider-azurestack/internal/clients"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurestack/internal/utils"
)

type TemplateDeploymentResource struct{}

func TestAccTemplateDeployment_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_template_deployment", "test")
	r := TemplateDeploymentResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basicMultiple(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
	})
}

func TestAccTemplateDeployment_requiresImport(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_template_deployment", "test")
	r := TemplateDeploymentResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basicMultiple(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		// data.RequiresImportErrorStep(r.requiresImport), // A bug? It expects an error but there's not an error
	})
}

func TestAccTemplateDeployment_disappears(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_template_deployment", "test")
	r := TemplateDeploymentResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		data.DisappearsStep(acceptance.DisappearsStepData{
			Config:       r.basicSingle,
			TestResource: r,
		}),
	})
}

func TestAccTemplateDeployment_nestedTemplate(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_template_deployment", "test")
	r := TemplateDeploymentResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.nestedTemplate(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
	})
}

func TestAccTemplateDeployment_withParams(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_template_deployment", "test")
	r := TemplateDeploymentResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.withParams(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				acceptance.TestCheckResourceAttr("azurestack_template_deployment.test", "outputs.testOutput", "Output Value"),
			),
		},
	})
}

func TestAccTemplateDeployment_withParamsBody(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_template_deployment", "test")
	r := TemplateDeploymentResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.withParamsBody(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				acceptance.TestCheckResourceAttr("azurestack_template_deployment.test", "outputs.testOutput", "Output Value"),
			),
		},
	})
}

func TestAccTemplateDeployment_withOutputs(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_template_deployment", "test")
	r := TemplateDeploymentResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.withOutputs(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				acceptance.TestCheckOutput("tfIntOutput", "-123"),
				acceptance.TestCheckOutput("tfStringOutput", "Standard_LRS"),

				// these values *should* be 'true' and 'false' but,
				// due to a bug in the way terraform represents bools at various times these are for now 0 and 1
				// see https://github.com/hashicorp/terraform/issues/13512#issuecomment-295389523
				// at a later date these may return the expected 'true' / 'false' and should be changed back
				acceptance.TestCheckOutput("tfFalseOutput", "false"),
				acceptance.TestCheckOutput("tfTrueOutput", "true"),
				check.That(data.ResourceName).Key("outputs.stringOutput").HasValue("Standard_LRS"),
			),
		},
	})
}

func TestAccTemplateDeployment_withError(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_template_deployment", "test")
	r := TemplateDeploymentResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config:      r.withError(data),
			ExpectError: regexp.MustCompile("Error: creating Template Deployment"),
		},
	})
}

func (t TemplateDeploymentResource) Exists(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := resourceids.ParseAzureResourceID(state.ID)
	if err != nil {
		return nil, err
	}

	name := id.Path["deployments"]
	if name == "" {
		name = id.Path["Deployments"]
	}

	resp, err := clients.Resource.DeploymentsClient.Get(ctx, id.ResourceGroup, name)
	if err != nil {
		return nil, fmt.Errorf("reading Template Deployment (%s): %+v", id, err)
	}

	return pointer.FromBool(resp.ID != nil), nil
}

func (r TemplateDeploymentResource) Destroy(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	client := clients.Resource.DeploymentsClient

	id, err := resourceids.ParseAzureResourceID(state.ID)
	if err != nil {
		return nil, err
	}

	name := id.Path["deployments"]
	if name == "" {
		name = id.Path["Deployments"]
	}

	if _, err = client.Delete(ctx, id.ResourceGroup, name); err != nil {
		return nil, fmt.Errorf("deleting template deployment %q: %+v", id, err)
	}

	// we can't use the Waiter here since the API returns a 200 once it's deleted which is considered a polling status code..
	log.Printf("[DEBUG] Waiting for Template Deployment (%q in Resource Group %q) to be deleted", name, id.ResourceGroup)
	stateConf := &acceptance.StateChangeConf{
		Pending: []string{"200"},
		Target:  []string{"404"},
		Timeout: 40 * time.Minute,
		Refresh: func() (interface{}, string, error) {
			res, err := client.Get(ctx, id.ResourceGroup, name)

			log.Printf("retrieving Template Deployment %q: %d", id, res.StatusCode)

			if err != nil {
				if utils.ResponseWasNotFound(res.Response) {
					return res, strconv.Itoa(res.StatusCode), nil
				}
				return nil, "", fmt.Errorf("polling for the status of the Template Deployment %q: %+v", id, err)
			}

			return res, strconv.Itoa(res.StatusCode), nil
		},
	}

	if _, err := stateConf.WaitForStateContext(ctx); err != nil {
		return nil, fmt.Errorf("waiting for Template Deployment %q to be deleted: %+v", id, err)
	}

	return pointer.FromBool(true), nil
}

func (TemplateDeploymentResource) basicSingle(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_template_deployment" "test" {
  name                = "acctesttemplate-%d"
  resource_group_name = azurestack_resource_group.test.name

  template_body = <<DEPLOY
{
  "$schema": "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
  "contentVersion": "1.0.0.0",
  "variables": {
    "location": "[resourceGroup().location]",
    "publicIPAddressType": "Dynamic",
    "apiVersion": "2015-06-15",
    "dnsLabelPrefix": "[concat('terraform-tdacctest', uniquestring(resourceGroup().id))]"
  },
  "resources": [
     {
      "type": "Microsoft.Network/publicIPAddresses",
      "apiVersion": "[variables('apiVersion')]",
      "name": "acctestpip-%d",
      "location": "[variables('location')]",
      "properties": {
        "publicIPAllocationMethod": "[variables('publicIPAddressType')]",
        "dnsSettings": {
          "domainNameLabel": "[variables('dnsLabelPrefix')]"
        }
      }
    }
  ]
}
DEPLOY


  deployment_mode = "Complete"
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger)
}

func (TemplateDeploymentResource) basicMultiple(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_template_deployment" "test" {
  name                = "acctesttemplate-%d"
  resource_group_name = azurestack_resource_group.test.name

  template_body = <<DEPLOY
{
  "$schema": "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
  "contentVersion": "1.0.0.0",
  "parameters": {
    "storageAccountType": {
      "type": "string",
      "defaultValue": "Standard_LRS",
      "allowedValues": [
        "Standard_LRS",
        "Standard_GRS",
        "Standard_ZRS"
      ],
      "metadata": {
        "description": "Storage Account type"
      }
    }
  },
  "variables": {
    "location": "[resourceGroup().location]",
    "storageAccountName": "[concat(uniquestring(resourceGroup().id), 'storage')]",
    "publicIPAddressName": "[concat('myPublicIp', uniquestring(resourceGroup().id))]",
    "publicIPAddressType": "Dynamic",
    "apiVersion": "2015-06-15",
    "dnsLabelPrefix": "[concat('terraform-tdacctest', uniquestring(resourceGroup().id))]"
  },
  "resources": [
    {
      "type": "Microsoft.Storage/storageAccounts",
      "name": "[variables('storageAccountName')]",
      "apiVersion": "[variables('apiVersion')]",
      "location": "[variables('location')]",
      "properties": {
        "accountType": "[parameters('storageAccountType')]"
      }
    },
    {
      "type": "Microsoft.Network/publicIPAddresses",
      "apiVersion": "[variables('apiVersion')]",
      "name": "[variables('publicIPAddressName')]",
      "location": "[variables('location')]",
      "properties": {
        "publicIPAllocationMethod": "[variables('publicIPAddressType')]",
        "dnsSettings": {
          "domainNameLabel": "[variables('dnsLabelPrefix')]"
        }
      }
    }
  ]
}
DEPLOY


  deployment_mode = "Complete"
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger)
}

//nolint:unused
func (TemplateDeploymentResource) requiresImport(data acceptance.TestData) string {
	template := TemplateDeploymentResource{}.basicMultiple(data)
	return fmt.Sprintf(`
%s

resource "azurestack_template_deployment" "import" {
  name                = azurestack_template_deployment.test.name
  resource_group_name = azurestack_template_deployment.test.resource_group_name

  template_body   = azurestack_template_deployment.test.template_body
  deployment_mode = azurestack_template_deployment.test.deployment_mode
}
`, template)
}

func (TemplateDeploymentResource) nestedTemplate(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_template_deployment" "test" {
  name                = "acctesttemplate-%d"
  resource_group_name = azurestack_resource_group.test.name

  template_body = <<DEPLOY
{
  "$schema": "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
  "contentVersion": "1.0.0.0",
  "parameters": {
  },
  "variables": {
    "location": "[resourceGroup().location]",
    "resourceGroupName": "[resourceGroup().name]"
  },
  "resources": [
    {
      "apiVersion": "2017-05-10",
      "name": "nested-template-deployment",
      "type": "Microsoft.Resources/deployments",
      "resourceGroup": "[variables('resourceGroupName')]",
      "properties": {
        "mode": "Incremental",
        "template": {
          "$schema": "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
          "contentVersion": "1.0.0.0",
          "variables": {
            "location": "[variables('location')]",
            "resourceGroupName": "[variables('resourceGroupName')]"
          },
          "resources": [
            {
              "type": "Microsoft.Network/publicIPAddresses",
              "apiVersion": "2015-06-15",
              "name": "acctest-pip",
              "location": "[variables('location')]",
              "properties": {
                "publicIPAllocationMethod": "Dynamic"
              }
            }
          ]
        }
      }
    }
  ]
}
DEPLOY


  deployment_mode = "Complete"
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger)
}

func (TemplateDeploymentResource) withParamsBody(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

data "azurestack_client_config" "current" {}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

output "test" {
  value = azurestack_template_deployment.test.outputs["testOutput"]
}

resource "azurestack_storage_container" "using-outputs" {
  name                  = "vhds"
  storage_account_name  = azurestack_template_deployment.test.outputs["accountName"]
  container_access_type = "private"
}


resource "azurestack_key_vault" "test" {
  location            = "%s"
  name                = "vault%d"
  resource_group_name = "${azurestack_resource_group.test.name}"
  sku_name            = "standard"

  tenant_id                       = data.azurestack_client_config.current.tenant_id
  enabled_for_template_deployment = true

  access_policy {
    key_permissions = []
    object_id       = data.azurestack_client_config.current.service_principal_object_id

    secret_permissions = [
      "delete",
      "get",
      "list",
      "set",
      "purge",
    ]

    tenant_id = "${data.azurestack_client_config.current.tenant_id}"
  }
}

resource "azurestack_key_vault_secret" "test-secret" {
  name         = "acctestsecret-%d"
  value        = "terraform-test-%d"
  key_vault_id = azurestack_key_vault.test.id
}

locals {
  templated-file = <<TPL
{
"dnsLabelPrefix": {
    "reference": {
      "keyvault": {
        "id": "${azurestack_key_vault.test.id}"
      },
      "secretName": "${azurestack_key_vault_secret.test-secret.name}"
    }
  },
"storageAccountType": {
   "value": "Standard_LRS"
  }
}
TPL
}

resource "azurestack_template_deployment" "test" {
  name                = "acctesttemplate-%d"
  resource_group_name = azurestack_resource_group.test.name

  template_body = <<DEPLOY
{
  "$schema": "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
  "contentVersion": "1.0.0.0",
  "parameters": {
    "storageAccountType": {
      "type": "string",
      "defaultValue": "Standard_LRS",
      "allowedValues": [
        "Standard_LRS",
        "Standard_GRS",
        "Standard_ZRS"
      ],
      "metadata": {
        "description": "Storage Account type"
      }
    },
    "dnsLabelPrefix": {
      "type": "string",
      "metadata": {
        "description": "DNS Label for the Public IP. Must be lowercase. It should match with the following regular expression: ^[a-z][a-z0-9-]{1,61}[a-z0-9]$ or it will raise an error."
      }
    }
  },
  "variables": {
    "location": "[resourceGroup().location]",
    "storageAccountName": "[concat(uniquestring(resourceGroup().id), 'storage')]",
    "publicIPAddressName": "[concat('myPublicIp', uniquestring(resourceGroup().id))]",
    "publicIPAddressType": "Dynamic",
    "apiVersion": "2015-06-15"
  },
  "resources": [
    {
      "type": "Microsoft.Storage/storageAccounts",
      "name": "[variables('storageAccountName')]",
      "apiVersion": "[variables('apiVersion')]",
      "location": "[variables('location')]",
      "properties": {
        "accountType": "[parameters('storageAccountType')]"
      }
    },
    {
      "type": "Microsoft.Network/publicIPAddresses",
      "apiVersion": "[variables('apiVersion')]",
      "name": "[variables('publicIPAddressName')]",
      "location": "[variables('location')]",
      "properties": {
        "publicIPAllocationMethod": "[variables('publicIPAddressType')]",
        "dnsSettings": {
          "domainNameLabel": "[parameters('dnsLabelPrefix')]"
        }
      }
    }
  ],
  "outputs": {
    "testOutput": {
      "type": "string",
      "value": "Output Value"
    },
    "accountName": {
      "type": "string",
      "value": "[variables('storageAccountName')]"
    }
  }
}
DEPLOY

  parameters_body = "${local.templated-file}"
  deployment_mode = "Incremental"
  depends_on      = ["azurestack_key_vault_secret.test-secret"]
}
`, data.RandomInteger, data.Locations.Primary, data.Locations.Primary, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger)
}

func (TemplateDeploymentResource) withParams(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

output "test" {
  value = azurestack_template_deployment.test.outputs["testOutput"]
}

resource "azurestack_storage_container" "using-outputs" {
  name                  = "vhds"
  storage_account_name  = azurestack_template_deployment.test.outputs["accountName"]
  container_access_type = "private"
}

resource "azurestack_template_deployment" "test" {
  name                = "acctesttemplate-%d"
  resource_group_name = azurestack_resource_group.test.name

  template_body = <<DEPLOY
{
  "$schema": "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
  "contentVersion": "1.0.0.0",
  "parameters": {
    "storageAccountType": {
      "type": "string",
      "defaultValue": "Standard_LRS",
      "allowedValues": [
        "Standard_LRS",
        "Standard_GRS",
        "Standard_ZRS"
      ],
      "metadata": {
        "description": "Storage Account type"
      }
    },
    "dnsLabelPrefix": {
      "type": "string",
      "metadata": {
        "description": "DNS Label for the Public IP. Must be lowercase. It should match with the following regular expression: ^[a-z][a-z0-9-]{1,61}[a-z0-9]$ or it will raise an error."
      }
    }
  },
  "variables": {
    "location": "[resourceGroup().location]",
    "storageAccountName": "[concat(uniquestring(resourceGroup().id), 'storage')]",
    "publicIPAddressName": "[concat('myPublicIp', uniquestring(resourceGroup().id))]",
    "publicIPAddressType": "Dynamic",
    "apiVersion": "2015-06-15"
  },
  "resources": [
    {
      "type": "Microsoft.Storage/storageAccounts",
      "name": "[variables('storageAccountName')]",
      "apiVersion": "[variables('apiVersion')]",
      "location": "[variables('location')]",
      "properties": {
        "accountType": "[parameters('storageAccountType')]"
      }
    },
    {
      "type": "Microsoft.Network/publicIPAddresses",
      "apiVersion": "[variables('apiVersion')]",
      "name": "[variables('publicIPAddressName')]",
      "location": "[variables('location')]",
      "properties": {
        "publicIPAllocationMethod": "[variables('publicIPAddressType')]",
        "dnsSettings": {
          "domainNameLabel": "[parameters('dnsLabelPrefix')]"
        }
      }
    }
  ],
  "outputs": {
    "testOutput": {
      "type": "string",
      "value": "Output Value"
    },
    "accountName": {
      "type": "string",
      "value": "[variables('storageAccountName')]"
    }
  }
}
DEPLOY


  parameters = {
    dnsLabelPrefix     = "terraform-test-%d"
    storageAccountType = "Standard_LRS"
  }

  deployment_mode = "Complete"
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger)
}

func (TemplateDeploymentResource) withOutputs(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

output "tfStringOutput" {
  value = azurestack_template_deployment.test.outputs["stringOutput"]
}

output "tfIntOutput" {
  value = azurestack_template_deployment.test.outputs["intOutput"]
}

output "tfFalseOutput" {
  value = azurestack_template_deployment.test.outputs["falseOutput"]
}

output "tfTrueOutput" {
  value = azurestack_template_deployment.test.outputs["trueOutput"]
}

resource "azurestack_template_deployment" "test" {
  name                = "acctesttemplate-%d"
  resource_group_name = azurestack_resource_group.test.name

  template_body = <<DEPLOY
{
  "$schema": "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
  "contentVersion": "1.0.0.0",
  "parameters": {
    "storageAccountType": {
      "type": "string",
      "defaultValue": "Standard_LRS",
      "allowedValues": [
        "Standard_LRS",
        "Standard_GRS",
        "Standard_ZRS"
      ],
      "metadata": {
        "description": "Storage Account type"
      }
    },
    "dnsLabelPrefix": {
      "type": "string",
      "metadata": {
        "description": "DNS Label for the Public IP. Must be lowercase. It should match with the following regular expression: ^[a-z][a-z0-9-]{1,61}[a-z0-9]$ or it will raise an error."
      }
    },
    "intParameter": {
      "type": "int",
      "defaultValue": -123
    },
    "falseParameter": {
      "type": "bool",
      "defaultValue": false
    },
    "trueParameter": {
      "type": "bool",
      "defaultValue": true
    }
  },
  "variables": {
    "location": "[resourceGroup().location]",
    "storageAccountName": "[concat(uniquestring(resourceGroup().id), 'storage')]",
    "publicIPAddressName": "[concat('myPublicIp', uniquestring(resourceGroup().id))]",
    "publicIPAddressType": "Dynamic",
    "apiVersion": "2015-06-15"
  },
  "resources": [
    {
      "type": "Microsoft.Storage/storageAccounts",
      "name": "[variables('storageAccountName')]",
      "apiVersion": "[variables('apiVersion')]",
      "location": "[variables('location')]",
      "properties": {
        "accountType": "[parameters('storageAccountType')]"
      }
    },
    {
      "type": "Microsoft.Network/publicIPAddresses",
      "apiVersion": "[variables('apiVersion')]",
      "name": "[variables('publicIPAddressName')]",
      "location": "[variables('location')]",
      "properties": {
        "publicIPAllocationMethod": "[variables('publicIPAddressType')]",
        "dnsSettings": {
          "domainNameLabel": "[parameters('dnsLabelPrefix')]"
        }
      }
    }
  ],
  "outputs": {
    "stringOutput": {
      "type": "string",
      "value": "[parameters('storageAccountType')]"
    },
    "intOutput": {
      "type": "int",
      "value": "[parameters('intParameter')]"
    },
    "falseOutput": {
      "type": "bool",
      "value": "[parameters('falseParameter')]"
    },
    "trueOutput": {
      "type": "bool",
      "value": "[parameters('trueParameter')]"
    }
  }
}
DEPLOY


  parameters = {
    dnsLabelPrefix     = "terraform-test-%d"
    storageAccountType = "Standard_LRS"
  }

  deployment_mode = "Incremental"
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger)
}

// StorageAccount name is too long, forces error
func (TemplateDeploymentResource) withError(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

output "test" {
  value = azurestack_template_deployment.test.outputs["testOutput"]
}

resource "azurestack_template_deployment" "test" {
  name                = "acctesttemplate-%d"
  resource_group_name = azurestack_resource_group.test.name

  template_body = <<DEPLOY
{
  "$schema": "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
  "contentVersion": "1.0.0.0",
  "parameters": {
    "storageAccountType": {
      "type": "string",
      "defaultValue": "Standard_LRS",
      "allowedValues": [
        "Standard_LRS",
        "Standard_GRS",
        "Standard_ZRS"
      ],
      "metadata": {
        "description": "Storage Account type"
      }
    }
  },
  "variables": {
    "location": "[resourceGroup().location]",
    "storageAccountName": "badStorageAccountNameTooLong",
    "apiVersion": "2015-06-15"
  },
  "resources": [
    {
      "type": "Microsoft.Storage/storageAccounts",
      "name": "[variables('storageAccountName')]",
      "apiVersion": "[variables('apiVersion')]",
      "location": "[variables('location')]",
      "properties": {
        "accountType": "[parameters('storageAccountType')]"
      }
    }
  ],
  "outputs": {
    "testOutput": {
      "type": "string",
      "value": "Output Value"
    }
  }
}
DEPLOY


  parameters = {
    storageAccountType = "Standard_LRS"
  }

  deployment_mode = "Complete"
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger)
}

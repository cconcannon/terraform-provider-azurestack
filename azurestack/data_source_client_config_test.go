package azurestack

import (
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
)

func TestAccDataSourceAzureRMClientConfig_basic(t *testing.T) {
	dataSourceName := "data.azurestack_client_config.current"
	clientId := os.Getenv("ARM_CLIENT_ID")
	tenantId := os.Getenv("ARM_TENANT_ID")
	subscriptionId := os.Getenv("ARM_SUBSCRIPTION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckArmClientConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAzureRMClientConfigAttr(dataSourceName, "client_id", clientId),
					testAzureRMClientConfigAttr(dataSourceName, "tenant_id", tenantId),
					testAzureRMClientConfigAttr(dataSourceName, "subscription_id", subscriptionId),
					testAzureRMClientConfigGUIDAttr(dataSourceName, "service_principal_application_id"),
					testAzureRMClientConfigGUIDAttr(dataSourceName, "service_principal_object_id"),
				),
			},
		},
	})
}

// Wraps resource.TestCheckResourceAttr to prevent leaking values to console
// in case of mismatch
func testAzureRMClientConfigAttr(name, key, value string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		err := resource.TestCheckResourceAttr(name, key, value)(s)
		if err != nil {
			// return fmt.Errorf("%s: Attribute '%s', failed check (values hidden)", name, key)
			return err
		}

		return nil
	}
}

func testAzureRMClientConfigGUIDAttr(name, key string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		r, err := regexp.Compile("^[A-Fa-f0-9]{8}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{12}$")
		if err != nil {
			return err
		}

		err = resource.TestMatchResourceAttr(name, key, r)(s)
		if err != nil {
			return err
		}

		return nil
	}
}

const testAccCheckArmClientConfig_basic = `
data "azurestack_client_config" "current" { }
`

package dns_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/profiles/2020-09-01/dns/mgmt/dns"
	"github.com/hashicorp/terraform-provider-azurestack/internal/clients"
	"github.com/hashicorp/terraform-provider-azurestack/internal/services/dns/parse"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurestack/internal/utils"
)

type DnsMxRecordResource struct{}

func TestAccDnsMxRecord_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_dns_mx_record", "test")
	r := DnsMxRecordResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				check.That(data.ResourceName).Key("fqdn").Exists(),
			),
		},
		data.ImportStep(),
	})
}

func TestAccDnsMxRecord_rootrecord(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_dns_mx_record", "test")
	r := DnsMxRecordResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.rootrecord(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				check.That(data.ResourceName).Key("fqdn").Exists(),
			),
		},
		data.ImportStep(),
	})
}

func TestAccDnsMxRecord_requiresImport(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_dns_mx_record", "test")
	r := DnsMxRecordResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		{
			Config:      r.requiresImport(data),
			ExpectError: acceptance.RequiresImportError("azurestack_dns_mx_record"),
		},
	})
}

func TestAccDnsMxRecord_updateRecords(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_dns_mx_record", "test")
	r := DnsMxRecordResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				check.That(data.ResourceName).Key("record.#").HasValue("2"),
			),
		},
		{
			Config: r.updateRecords(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				check.That(data.ResourceName).Key("record.#").HasValue("3"),
			),
		},
	})
}

func TestAccDnsMxRecord_withTags(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurestack_dns_mx_record", "test")
	r := DnsMxRecordResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.withTags(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				check.That(data.ResourceName).Key("tags.%").HasValue("2"),
			),
		},
		{
			Config: r.withTagsUpdate(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				check.That(data.ResourceName).Key("tags.%").HasValue("1"),
			),
		},
		data.ImportStep(),
	})
}

func (DnsMxRecordResource) Exists(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := parse.MxRecordID(state.ID)
	if err != nil {
		return nil, err
	}

	resp, err := clients.Dns.RecordSetsClient.Get(ctx, id.ResourceGroup, id.DnszoneName, id.MXName, dns.MX)
	if err != nil {
		return nil, fmt.Errorf("retrieving DNS MX record %s (resource group: %s): %v", id.MXName, id.ResourceGroup, err)
	}

	return utils.Bool(resp.RecordSetProperties != nil), nil
}

func (DnsMxRecordResource) basic(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_dns_zone" "test" {
  name                = "acctestzone%d.com"
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_dns_mx_record" "test" {
  name                = "myarecord%d"
  resource_group_name = azurestack_resource_group.test.name
  zone_name           = azurestack_dns_zone.test.name
  ttl                 = 300

  record {
    preference = "10"
    exchange   = "mail1.contoso.com"
  }

  record {
    preference = "20"
    exchange   = "mail2.contoso.com"
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger)
}

func (DnsMxRecordResource) rootrecord(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_dns_zone" "test" {
  name                = "acctestzone%d.com"
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_dns_mx_record" "test" {
  resource_group_name = azurestack_resource_group.test.name
  zone_name           = azurestack_dns_zone.test.name
  ttl                 = 300

  record {
    preference = "10"
    exchange   = "mail1.contoso.com"
  }

  record {
    preference = "20"
    exchange   = "mail2.contoso.com"
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger)
}

func (r DnsMxRecordResource) requiresImport(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurestack_dns_mx_record" "import" {
  name                = azurestack_dns_mx_record.test.name
  resource_group_name = azurestack_dns_mx_record.test.resource_group_name
  zone_name           = azurestack_dns_mx_record.test.zone_name
  ttl                 = 300

  record {
    preference = "10"
    exchange   = "mail1.contoso.com"
  }

  record {
    preference = "20"
    exchange   = "mail2.contoso.com"
  }
}
`, r.basic(data))
}

func (DnsMxRecordResource) updateRecords(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_dns_zone" "test" {
  name                = "acctestzone%d.com"
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_dns_mx_record" "test" {
  name                = "myarecord%d"
  resource_group_name = azurestack_resource_group.test.name
  zone_name           = azurestack_dns_zone.test.name
  ttl                 = 300

  record {
    preference = "10"
    exchange   = "mail1.contoso.com"
  }

  record {
    preference = "20"
    exchange   = "mail2.contoso.com"
  }

  record {
    preference = "50"
    exchange   = "mail3.contoso.com"
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger)
}

func (DnsMxRecordResource) withTags(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_dns_zone" "test" {
  name                = "acctestzone%d.com"
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_dns_mx_record" "test" {
  name                = "myarecord%d"
  resource_group_name = azurestack_resource_group.test.name
  zone_name           = azurestack_dns_zone.test.name
  ttl                 = 300

  record {
    preference = "10"
    exchange   = "mail1.contoso.com"
  }

  record {
    preference = "20"
    exchange   = "mail2.contoso.com"
  }

  tags = {
    environment = "Production"
    cost_center = "MSFT"
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger)
}

func (DnsMxRecordResource) withTagsUpdate(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_dns_zone" "test" {
  name                = "acctestzone%d.com"
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_dns_mx_record" "test" {
  name                = "myarecord%d"
  resource_group_name = azurestack_resource_group.test.name
  zone_name           = azurestack_dns_zone.test.name
  ttl                 = 300

  record {
    preference = "10"
    exchange   = "mail1.contoso.com"
  }

  record {
    preference = "20"
    exchange   = "mail2.contoso.com"
  }

  tags = {
    environment = "staging"
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger)
}

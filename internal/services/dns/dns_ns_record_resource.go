package dns

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/2020-09-01/dns/mgmt/dns"
	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/terraform-provider-azurestack/internal/az/tags"
	"github.com/hashicorp/terraform-provider-azurestack/internal/clients"
	"github.com/hashicorp/terraform-provider-azurestack/internal/services/dns/parse"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/timeouts"
	"github.com/hashicorp/terraform-provider-azurestack/internal/utils"
)

func dnsNsRecord() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: dnsNsRecordCreate,
		Read:   dnsNsRecordRead,
		Update: dnsNsRecordUpdate,
		Delete: dnsNsRecordDelete,

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},
		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			_, err := parse.NsRecordID(id)
			return err
		}),

		Schema: map[string]*pluginsdk.Schema{
			"name": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
			},

			"resource_group_name": commonschema.ResourceGroupName(),

			"zone_name": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
			},

			"records": {
				Type:     pluginsdk.TypeList,
				Required: true,
				Elem: &pluginsdk.Schema{
					Type: pluginsdk.TypeString,
				},
			},

			"ttl": {
				Type:     pluginsdk.TypeInt,
				Required: true,
			},

			"fqdn": {
				Type:     pluginsdk.TypeString,
				Computed: true,
			},

			"tags": tags.Schema(),
		},
	}
}

func dnsNsRecordCreate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Dns.RecordSetsClient
	ctx, cancel := timeouts.ForCreate(meta.(*clients.Client).StopContext, d)
	subscriptionId := meta.(*clients.Client).Account.SubscriptionId
	defer cancel()

	name := d.Get("name").(string)
	resGroup := d.Get("resource_group_name").(string)
	zoneName := d.Get("zone_name").(string)

	resourceId := parse.NewNsRecordID(subscriptionId, resGroup, zoneName, name)

	existing, err := client.Get(ctx, resGroup, zoneName, name, dns.NS)
	if err != nil {
		if !utils.ResponseWasNotFound(existing.Response) {
			return fmt.Errorf("checking for presence of existing DNS NS Record %q (Zone %q / Resource Group %q): %s", name, zoneName, resGroup, err)
		}
	}

	if !utils.ResponseWasNotFound(existing.Response) {
		return tf.ImportAsExistsError("azurestack_dns_ns_record", resourceId.ID())
	}

	ttl := int64(d.Get("ttl").(int))
	t := d.Get("tags").(map[string]interface{})

	recordsRaw := d.Get("records").([]interface{})
	records := expandazurestackDnsNsRecords(recordsRaw)

	parameters := dns.RecordSet{
		Name: &name,
		RecordSetProperties: &dns.RecordSetProperties{
			Metadata:  tags.Expand(t),
			TTL:       &ttl,
			NsRecords: records,
		},
	}

	eTag := ""
	ifNoneMatch := "" // set to empty to allow updates to records after creation
	if _, err := client.CreateOrUpdate(ctx, resGroup, zoneName, name, dns.NS, parameters, eTag, ifNoneMatch); err != nil {
		return fmt.Errorf("creating DNS NS Record %q (Zone %q / Resource Group %q): %s", name, zoneName, resGroup, err)
	}

	d.SetId(resourceId.ID())

	return dnsNsRecordRead(d, meta)
}

func dnsNsRecordUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Dns.RecordSetsClient
	ctx, cancel := timeouts.ForUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.NsRecordID(d.Id())
	if err != nil {
		return err
	}

	existing, err := client.Get(ctx, id.ResourceGroup, id.DnszoneName, id.NSName, dns.NS)
	if err != nil {
		return fmt.Errorf("retrieving NS %q (DNS Zone %q / Resource Group %q): %+v", id.NSName, id.DnszoneName, id.ResourceGroup, err)
	}

	if existing.RecordSetProperties == nil {
		return fmt.Errorf("retrieving NS %q (DNS Zone %q / Resource Group %q): `properties` was nil", id.NSName, id.DnszoneName, id.ResourceGroup)
	}

	if d.HasChange("records") {
		recordsRaw := d.Get("records").([]interface{})
		records := expandazurestackDnsNsRecords(recordsRaw)
		existing.RecordSetProperties.NsRecords = records
	}

	if d.HasChange("tags") {
		t := d.Get("tags").(map[string]interface{})
		existing.RecordSetProperties.Metadata = tags.Expand(t)
	}

	if d.HasChange("ttl") {
		existing.RecordSetProperties.TTL = pointer.FromInt64(int64(d.Get("ttl").(int)))
	}

	eTag := ""
	ifNoneMatch := "" // set to empty to allow updates to records after creation
	if _, err := client.CreateOrUpdate(ctx, id.ResourceGroup, id.DnszoneName, id.NSName, dns.NS, existing, eTag, ifNoneMatch); err != nil {
		return fmt.Errorf("updating DNS NS Record %q (Zone %q / Resource Group %q): %s", id.NSName, id.DnszoneName, id.ResourceGroup, err)
	}

	return dnsNsRecordRead(d, meta)
}

func dnsNsRecordRead(d *pluginsdk.ResourceData, meta interface{}) error {
	dnsClient := meta.(*clients.Client).Dns.RecordSetsClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.NsRecordID(d.Id())
	if err != nil {
		return err
	}

	resp, err := dnsClient.Get(ctx, id.ResourceGroup, id.DnszoneName, id.NSName, dns.NS)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("reading DNS NS record %s: %+v", id.NSName, err)
	}

	d.Set("name", id.NSName)
	d.Set("resource_group_name", id.ResourceGroup)
	d.Set("zone_name", id.DnszoneName)

	d.Set("ttl", resp.TTL)
	d.Set("fqdn", resp.Fqdn)

	if props := resp.RecordSetProperties; props != nil {
		if err := d.Set("records", flattenazurestackDnsNsRecords(props.NsRecords)); err != nil {
			return fmt.Errorf("settings `records`: %+v", err)
		}
	}

	return tags.FlattenAndSet(d, resp.Metadata)
}

func dnsNsRecordDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	dnsClient := meta.(*clients.Client).Dns.RecordSetsClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.NsRecordID(d.Id())
	if err != nil {
		return err
	}

	resp, err := dnsClient.Delete(ctx, id.ResourceGroup, id.DnszoneName, id.NSName, dns.NS, "")
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deleting DNS NS Record %s: %+v", id.NSName, err)
	}

	return nil
}

func flattenazurestackDnsNsRecords(records *[]dns.NsRecord) []interface{} {
	if records == nil {
		return []interface{}{}
	}

	results := make([]interface{}, 0)
	for _, record := range *records {
		if record.Nsdname == nil {
			continue
		}

		results = append(results, *record.Nsdname)
	}

	return results
}

func expandazurestackDnsNsRecords(input []interface{}) *[]dns.NsRecord {
	records := make([]dns.NsRecord, len(input))
	for i, v := range input {
		record := v.(string)

		nsRecord := dns.NsRecord{
			Nsdname: &record,
		}

		records[i] = nsRecord
	}
	return &records
}

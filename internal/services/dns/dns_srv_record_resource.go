package dns

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/2020-09-01/dns/mgmt/dns"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/terraform-provider-azurestack/internal/az/tags"
	"github.com/hashicorp/terraform-provider-azurestack/internal/clients"
	"github.com/hashicorp/terraform-provider-azurestack/internal/services/dns/parse"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/timeouts"
	"github.com/hashicorp/terraform-provider-azurestack/internal/utils"
)

func dnsSrvRecord() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: dnsSrvRecordCreateUpdate,
		Read:   dnsSrvRecordRead,
		Update: dnsSrvRecordCreateUpdate,
		Delete: dnsSrvRecordDelete,

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},
		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			_, err := parse.SrvRecordID(id)
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
			},

			"record": {
				Type:     pluginsdk.TypeSet,
				Required: true,
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"priority": {
							Type:     pluginsdk.TypeInt,
							Required: true,
						},

						"weight": {
							Type:     pluginsdk.TypeInt,
							Required: true,
						},

						"port": {
							Type:     pluginsdk.TypeInt,
							Required: true,
						},

						"target": {
							Type:     pluginsdk.TypeString,
							Required: true,
						},
					},
				},
				Set: dnsSrvRecordHash,
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

func dnsSrvRecordCreateUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Dns.RecordSetsClient
	ctx, cancel := timeouts.ForCreateUpdate(meta.(*clients.Client).StopContext, d)
	subscriptionId := meta.(*clients.Client).Account.SubscriptionId
	defer cancel()

	name := d.Get("name").(string)
	resGroup := d.Get("resource_group_name").(string)
	zoneName := d.Get("zone_name").(string)

	resourceId := parse.NewSrvRecordID(subscriptionId, resGroup, zoneName, name)

	if d.IsNewResource() {
		existing, err := client.Get(ctx, resGroup, zoneName, name, dns.SRV)
		if err != nil {
			if !utils.ResponseWasNotFound(existing.Response) {
				return fmt.Errorf("checking for presence of existing DNS SRV Record %q (Zone %q / Resource Group %q): %s", name, zoneName, resGroup, err)
			}
		}

		if !utils.ResponseWasNotFound(existing.Response) {
			return tf.ImportAsExistsError("azurestack_dns_srv_record", resourceId.ID())
		}
	}

	ttl := int64(d.Get("ttl").(int))
	t := d.Get("tags").(map[string]interface{})

	parameters := dns.RecordSet{
		Name: &name,
		RecordSetProperties: &dns.RecordSetProperties{
			Metadata:   tags.Expand(t),
			TTL:        &ttl,
			SrvRecords: expandazurestackDnsSrvRecords(d),
		},
	}

	eTag := ""
	ifNoneMatch := "" // set to empty to allow updates to records after creation
	if _, err := client.CreateOrUpdate(ctx, resGroup, zoneName, name, dns.SRV, parameters, eTag, ifNoneMatch); err != nil {
		return fmt.Errorf("creating/updating DNS SRV Record %q (Zone %q / Resource Group %q): %s", name, zoneName, resGroup, err)
	}

	d.SetId(resourceId.ID())

	return dnsSrvRecordRead(d, meta)
}

func dnsSrvRecordRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Dns.RecordSetsClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.SrvRecordID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.Get(ctx, id.ResourceGroup, id.DnszoneName, id.SRVName, dns.SRV)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("reading DNS SRV record %s: %v", id.SRVName, err)
	}

	d.Set("name", id.SRVName)
	d.Set("resource_group_name", id.ResourceGroup)
	d.Set("zone_name", id.DnszoneName)
	d.Set("ttl", resp.TTL)
	d.Set("fqdn", resp.Fqdn)

	if err := d.Set("record", flattenazurestackDnsSrvRecords(resp.SrvRecords)); err != nil {
		return err
	}
	return tags.FlattenAndSet(d, resp.Metadata)
}

func dnsSrvRecordDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Dns.RecordSetsClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.SrvRecordID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.Delete(ctx, id.ResourceGroup, id.DnszoneName, id.SRVName, dns.SRV, "")
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deleting DNS SRV Record %s: %+v", id.SRVName, err)
	}

	return nil
}

func flattenazurestackDnsSrvRecords(records *[]dns.SrvRecord) []map[string]interface{} {
	results := make([]map[string]interface{}, 0)

	if records != nil {
		for _, record := range *records {
			results = append(results, map[string]interface{}{
				"priority": *record.Priority,
				"weight":   *record.Weight,
				"port":     *record.Port,
				"target":   *record.Target,
			})
		}
	}

	return results
}

func expandazurestackDnsSrvRecords(d *pluginsdk.ResourceData) *[]dns.SrvRecord {
	recordStrings := d.Get("record").(*pluginsdk.Set).List()
	records := make([]dns.SrvRecord, len(recordStrings))

	for i, v := range recordStrings {
		record := v.(map[string]interface{})
		priority := int32(record["priority"].(int))
		weight := int32(record["weight"].(int))
		port := int32(record["port"].(int))
		target := record["target"].(string)

		srvRecord := dns.SrvRecord{
			Priority: &priority,
			Weight:   &weight,
			Port:     &port,
			Target:   &target,
		}

		records[i] = srvRecord
	}

	return &records
}

func dnsSrvRecordHash(v interface{}) int {
	var buf bytes.Buffer

	if m, ok := v.(map[string]interface{}); ok {
		buf.WriteString(fmt.Sprintf("%d-", m["priority"].(int)))
		buf.WriteString(fmt.Sprintf("%d-", m["weight"].(int)))
		buf.WriteString(fmt.Sprintf("%d-", m["port"].(int)))
		buf.WriteString(fmt.Sprintf("%s-", m["target"].(string)))
	}

	return pluginsdk.HashString(buf.String())
}

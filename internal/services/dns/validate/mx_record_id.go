package validate

// NOTE: this file is generated via 'go:generate' - manual changes will be overwritten

import (
	"fmt"

	"github.com/hashicorp/terraform-provider-azurestack/internal/services/dns/parse"
)

func MxRecordID(input interface{}, key string) (warnings []string, errors []error) {
	v, ok := input.(string)
	if !ok {
		errors = append(errors, fmt.Errorf("expected %q to be a string", key))
		return
	}

	if _, err := parse.MxRecordID(v); err != nil {
		errors = append(errors, err)
	}

	return
}

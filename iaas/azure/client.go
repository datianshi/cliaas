package azure

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/arm/compute"
	"github.com/Azure/azure-sdk-for-go/arm/examples/helpers"
	"github.com/Azure/go-autorest/autorest"
	errwrap "github.com/pkg/errors"
)

const defaultResourceManagerEndpoint = "https://management.azure.com/"

type Client struct {
	VirtualMachinesClient ComputeVirtualMachinesClient
	resourceGroupName     string
}

type ComputeVirtualMachinesClient interface {
	ListAllNextResults(lastResults compute.VirtualMachineListResult) (result compute.VirtualMachineListResult, err error)
	CreateOrUpdate(resourceGroupName string, vmName string, parameters compute.VirtualMachine, cancel <-chan struct{}) (result autorest.Response, err error)
	Delete(resourceGroupName string, vmName string, cancel <-chan struct{}) (result autorest.Response, err error)
	Deallocate(resourceGroupName string, vmName string, cancel <-chan struct{}) (result autorest.Response, err error)
	List(resourceGroupName string) (result compute.VirtualMachineListResult, err error)
}

var InvalidAzureClientErr = errors.New("invalid azure sdk client defined")
var NoMatchesErr = errors.New("no VM names match the provided prefix")
var MultipleMatchesErr = errors.New("multiple VM names match the provided prefix")

func NewClient(
	subscriptionID string,
	clientID string,
	clientSecret string,
	tenantID string,
	resourceGroupName string,
	resourceManagerEndpoint string,
) (*Client, error) {
	c := map[string]string{
		"AZURE_CLIENT_ID":       clientID,
		"AZURE_CLIENT_SECRET":   clientSecret,
		"AZURE_SUBSCRIPTION_ID": subscriptionID,
		"AZURE_TENANT_ID":       tenantID,
	}
	if err := checkEnvVar(c); err != nil {
		return nil, errwrap.Wrap(err, "failed on check of env vars")
	}
	if resourceManagerEndpoint == "" {
		resourceManagerEndpoint = defaultResourceManagerEndpoint
	}

	spt, err := helpers.NewServicePrincipalTokenFromCredentials(c, resourceManagerEndpoint)
	if err != nil {
		return nil, errwrap.Wrap(err, "failed to generate new service principal token")
	}
	client := compute.NewVirtualMachinesClient(subscriptionID)
	client.Authorizer = spt
	return &Client{
		VirtualMachinesClient: &client,
		resourceGroupName:     resourceGroupName,
	}, nil
}

func (s *Client) Delete(identifier string) error {
	matchingInstances, err := s.getFilteredList(identifier)
	if err != nil {
		return errwrap.Wrap(err, "error when attempting to get filtered vm list")
	}

	switch len(matchingInstances) {
	case 0:
		return NoMatchesErr
	case 1:
		_, err = s.VirtualMachinesClient.Delete(s.resourceGroupName, *(matchingInstances[0].Name), nil)
		return err
	default:
		return MultipleMatchesErr
	}
}

func (s *Client) Replace(identifier string, vhdURL string) error {
	instance, err := s.deallocate(identifier)
	if err != nil {
		return errwrap.Wrap(err, "error shutting down VM")
	}

	tmpName := generateInstanceName(*instance.Name)
	instance.Name = &tmpName
	instance.VirtualMachineProperties.StorageProfile.OsDisk.Image.URI = &vhdURL
	_, err = s.VirtualMachinesClient.CreateOrUpdate(s.resourceGroupName, *instance.Name, *instance, nil)
	return err
}

func (s *Client) getFilteredList(identifier string) ([]compute.VirtualMachine, error) {
	vmListResults, err := s.VirtualMachinesClient.List(s.resourceGroupName)
	if err != nil {
		return nil, errwrap.Wrap(err, "error in getting list of VMs from azure")
	}

	var matchingInstances = make([]compute.VirtualMachine, 0)
	var vmNameFilter = regexp.MustCompile(identifier)

	for vmListResults.Value != nil && len(*vmListResults.Value) > 0 {
		matchingInstances = getMatchingInstances(*vmListResults.Value, vmNameFilter, matchingInstances)
		vmListResults, err = s.VirtualMachinesClient.ListAllNextResults(vmListResults)
		if err != nil {
			return nil, errwrap.Wrap(err, "ListAllNextResults call failed")
		}
	}
	return matchingInstances, nil
}

func (s *Client) deallocate(identifier string) (*compute.VirtualMachine, error) {
	matchingInstances, err := s.getFilteredList(identifier)
	if err != nil {
		return nil, errwrap.Wrap(err, "error when attempting to get filtered vm list")
	}

	switch len(matchingInstances) {
	case 0:
		return nil, NoMatchesErr
	case 1:
		_, err = s.VirtualMachinesClient.Deallocate(s.resourceGroupName, *matchingInstances[0].Name, nil)
		return &matchingInstances[0], err
	default:
		return nil, MultipleMatchesErr
	}
}

func checkEnvVar(envVars map[string]string) error {
	var missingVars []string
	for varName, value := range envVars {
		if value == "" {
			missingVars = append(missingVars, varName)
		}
	}
	if len(missingVars) > 0 {
		return fmt.Errorf("Missing environment variables %v", missingVars)
	}
	return nil
}

func generateInstanceName(currentName string) string {
	tstamp := time.Now().Format("20060112123456")
	splits := strings.Split(currentName, "_")
	if len(splits) == 1 {
		return currentName + "_" + tstamp
	}

	truncatedSplits := splits[:len(splits)-1]
	truncatedSplits = append(truncatedSplits, tstamp)
	return strings.Join(truncatedSplits, "_")
}

func getMatchingInstances(vmList []compute.VirtualMachine, identifierRegex *regexp.Regexp, matchingInstances []compute.VirtualMachine) []compute.VirtualMachine {

	for _, instance := range vmList {
		if identifierRegex.MatchString(*instance.Name) {
			matchingInstances = append(matchingInstances, instance)
		}
	}
	return matchingInstances
}
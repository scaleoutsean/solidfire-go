package cloudops

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/scaleoutsean/solidfire-go/sdk"
	"gopkg.in/yaml.v2"
)

const GiB = 1073741824

type Client struct {
	SFClient          *sdk.SFClient
	Endpoint          string
	URL               string
	Login             string
	Password          string
	Version           string
	SVIP              string
	DefaultVolumeSize string
	InitiatorIface    string
	TargetSecret      string
	InitiatorSecret   string
	TenantName        string
	AccountID         int64
	Limits            *sdk.GetLimitsResult
}

func parseEndpointString(ep string, c *Client) error {
	items := strings.Split(ep, "/")
	creds := strings.Split(items[2], "@")
	login := strings.Split(creds[0], ":")

	c.URL = creds[1]
	c.Login = login[0]
	c.Password = login[1]
	c.Version = items[4]
	return nil

}

func NewClient(c string) (*Client, error) {
	var client Client

	err := yaml.Unmarshal([]byte(c), &client)
	if err != nil {
		log.Printf("failure parsing supplied config yaml: %v\n", err)
		return &client, err
	}
	if parseEndpointString(client.Endpoint, &client) != nil {
		log.Printf("failure parsing endpoint string: %v\n", err)
		os.Exit(1)
	}

	var sf sdk.SFClient
	ctx := context.Background()
	sf.Connect(ctx, client.URL, client.Version, client.Login, client.Password)

	// We want to persist the connection info we created above, otherwise ever call is prefaced with
	// this connect routine (blek)
	client.SFClient = &sf

	if err != nil {
		log.Printf("failure verifying endpoint config while conducting initial client connection: %v\n", err)
		os.Exit(1)
	}
	if err := client.initAccount(ctx); err != nil {
		log.Printf("failed to initialize account: %v\n", err)
		os.Exit(1)
	}

	if client.InitiatorIface == "" {
		client.InitiatorIface = "default"
	}

	return &client, nil
}

func NewClientWithArgs(endpoint, version, tenantName string, defaultVolSize string) (*Client, error) {
	client := &Client{
		Endpoint:          endpoint,
		Version:           version,
		TenantName:        tenantName,
		DefaultVolumeSize: defaultVolSize,
	}

	if err := parseEndpointString(client.Endpoint, client); err != nil {
		log.Printf("failure parsing endpoint string: %v\n", err)
		return nil, err
	}

	var sf sdk.SFClient
	ctx := context.Background()
	sf.Connect(ctx, client.URL, client.Version, client.Login, client.Password)
	client.SFClient = &sf

	if err := client.initAccount(ctx); err != nil {
		return nil, err
	}
	if client.InitiatorIface == "" {
		client.InitiatorIface = "default"
	}
	return client, nil
}

func NewClientFromSecrets(url, user, password, version, tenantName, defaultVolSize string) (*Client, error) {
	client := &Client{
		URL:               url,
		Login:             user,
		Password:          password,
		Version:           version,
		TenantName:        tenantName,
		DefaultVolumeSize: defaultVolSize,
	}

	var sf sdk.SFClient
	ctx := context.Background()
	sf.Connect(ctx, client.URL, client.Version, client.Login, client.Password)
	client.SFClient = &sf

	if err := client.initAccount(ctx); err != nil {
		return nil, err
	}
	if client.InitiatorIface == "" {
		client.InitiatorIface = "default"
	}

	// Fetch Cluster Limits
	limits, limitErr := client.SFClient.GetLimits(ctx)
	if limitErr != nil {
		log.Printf("Warning: Failed to fetch cluster limits: %v\n", limitErr)
	} else {
		client.Limits = limits
	}

	return client, nil
}

func (c *Client) initAccount(ctx context.Context) error {
	// Verify specified user tenant/account and get it's ID, if it doesn't exist
	// create it
	req := sdk.GetAccountByNameRequest{}
	req.Username = c.TenantName
	var account sdk.Account
	result, sdkErr := c.SFClient.GetAccountByName(ctx, &req)
	if sdkErr != nil {
		// handle both specific xUnknownAccount or wrapped error containing it
		if strings.Contains(sdkErr.Detail, "xUnknownAccount") {
			req := sdk.AddAccountRequest{}
			req.Username = c.TenantName
			addResult, sdkErr := c.SFClient.AddAccount(ctx, &req)
			if sdkErr != nil {
				return fmt.Errorf("failed to create default account: %+v", sdkErr)
			}
			account = addResult.Account
		} else {
			return fmt.Errorf("error getting account: %+v", sdkErr)
		}
	} else {
		account = result.Account
	}
	c.AccountID = account.AccountID
	c.InitiatorSecret = account.InitiatorSecret
	c.TargetSecret = account.TargetSecret

	// Get Cluster Info to populate SVIP
	info, sdkErr := c.SFClient.GetClusterInfo(ctx)
	if sdkErr != nil {
		return fmt.Errorf("failed to get cluster info: %+v", sdkErr)
	}
	c.SVIP = info.ClusterInfo.Svip
	log.Printf("Fetched ClusterInfo. SVIP: %s, MVIP: %s", c.SVIP, info.ClusterInfo.Mvip)

	return nil
}

func (c *Client) GetCreateVolume(req sdk.CreateVolumeRequest) (*sdk.Volume, error) {
	ctx := context.Background()
	v, sdkErr := c.GetVolumeByName(req.Name)
	if sdkErr != nil {
	}

	if v != nil {
		return v, nil
	}

	vol, createErr := c.SFClient.CreateVolume(ctx, &req)
	if createErr != nil {
		log.Printf("failed to create volume: %+v\n", createErr)
		return &sdk.Volume{}, createErr
	}
	return &vol.Volume, nil
}

func (c *Client) DeleteVolume(volumeID int64) error {
	req := sdk.DeleteVolumeRequest{}
	req.VolumeID = volumeID
	ctx := context.Background()
	_, err := c.SFClient.DeleteVolume(ctx, &req)
	if err != nil {
		dneString := fmt.Sprintf("500:Volume %d does not exist.", req.VolumeID)
		if err.Detail == dneString {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) ExpandVolume(volumeID, newSize int64) error {
	req := sdk.ModifyVolumeRequest{}
	req.VolumeID = volumeID
	req.TotalSize = newSize * GiB

	ctx := context.Background()
	_, err := c.SFClient.ModifyVolume(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ModifyQoS(volumeID int64, qos *sdk.QoS) error {
	req := sdk.ModifyVolumeRequest{}
	req.VolumeID = volumeID
	req.Qos = qos

	ctx := context.Background()
	_, err := c.SFClient.ModifyVolume(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetVolumeByName(volumeName string) (*sdk.Volume, error) {
	req := sdk.ListVolumesForAccountRequest{}
	req.AccountID = c.AccountID

	ctx := context.Background()
	response, err := c.SFClient.ListVolumesForAccount(ctx, &req)
	if err != nil {
		return &sdk.Volume{}, err
	}
	// Removed incorrect return of first volume
	for _, v := range response.Volumes {
		// NOTE: Warning, I'm not checking for duplicate names which sadly is valid on SF
		if v.Name == volumeName {
			return &v, nil
		}
	}
	return nil, nil
}

func (c *Client) GetVolume(volumeID int64) (*sdk.Volume, error) {
	req := sdk.ListActiveVolumesRequest{}
	req.StartVolumeID = volumeID
	req.Limit = 1

	ctx := context.Background()
	response, err := c.SFClient.ListActiveVolumes(ctx, &req)
	if err != nil {
		return nil, err
	}
	if len(response.Volumes) > 0 {
		for _, v := range response.Volumes {
			if v.VolumeID == volumeID {
				if v.AccountID == c.AccountID {
					// We need to return a pointer to the value in the slice, but since v is a copy (range loop),
					// we can either return &v (which points to the stack local copy that might be unsafe if not copied properly)
					// or index into the slice.
					// Actually, range over slice returns a copy 'v'.
					// Safest is to return &response.Volumes[i] or return a copy.
					// Since struct is simple, returning &v is okay if we return *Volume.
					// Wait, Go 1.22+ changed loop variable semantics, but to be safe:
					vol := v
					return &vol, nil
				}
				// Found volume but wrong account?
				// This might be okay if we are admin? But we filter by account usually?
				// Actually ListActiveVolumes lists all volumes visible to the user.
				// If we are admin, we see all.
				// The check `v.AccountID == c.AccountID` implies we only want to see our own volumes.
				// If we want to allow admin to see everything, we should remove this check or make it optional.
				// But for now, let's keep it but Fix the Loop.
				return nil, fmt.Errorf("volume %d found but belongs to account %d (expected %d)", volumeID, v.AccountID, c.AccountID)
			}
		}
	}
	return nil, fmt.Errorf("volume %d not found", volumeID)
}

func (c *Client) ListVolumes() ([]sdk.Volume, error) {
	req := sdk.ListVolumesForAccountRequest{}
	req.AccountID = c.AccountID

	ctx := context.Background()
	response, err := c.SFClient.ListVolumesForAccount(ctx, &req)

	// FIX: Check for typed nil pointer trap
	// The generated SDK returns *SdkError. If the pointer is nil, accessing it is safe (it's address 0),
	// but assigning it to an `error` interface makes the interface non-nil.
	if err != nil {
		// Log the raw value/pointer to be absolutely sure
		return nil, fmt.Errorf("list volumes failed (code=%s): %s", err.Code, err.Detail)
	}
	if response == nil {
		return nil, fmt.Errorf("unexpected nil response from ListVolumesForAccount")
	}
	return response.Volumes, nil
}

func (c *Client) ConnectVolume(volumeID int64) (string, error) {
	v, err := c.GetVolume(volumeID)
	if err != nil {
		return "", err
	}
	path := "/dev/disk/by-path/ip-" + c.SVIP + "-iscsi-" + v.Iqn + "-lun-0"

	// Make sure it's not already attached
	if !sdk.WaitForPathToExist(path, 1) {
		err = sdk.LoginWithChap(v.Iqn, c.SVIP, c.TenantName, c.InitiatorSecret, c.InitiatorIface)
		if err != nil {
			return "", err
		}
	}

	if !sdk.WaitForPathToExist(path, 5) {
		return path, fmt.Errorf("failed to find device at path: %v\n", path)
	}
	return sdk.GetDeviceFileFromIscsiPath(path)
}

func (c *Client) CreateGroupSnapshot(volumes []int64, name string, enableRemoteReplication bool, ensureSerialCreation bool, retention string) (*sdk.CreateGroupSnapshotResult, error) {
	if len(volumes) < 2 {
		return nil, fmt.Errorf("CreateGroupSnapshot requires at least 2 volumes")
	}
	if len(volumes) > 32 {
		return nil, fmt.Errorf("CreateGroupSnapshot requires at most 32 volumes")
	}

	req := sdk.CreateGroupSnapshotRequest{
		Volumes:                 volumes,
		Name:                    name,
		EnableRemoteReplication: enableRemoteReplication,
		EnsureSerialCreation:    ensureSerialCreation,
	}
	// Default retention for consistency with single snapshots if not provided
	if retention != "" {
		req.Retention = retention
	} else {
		req.Retention = "24:00:00"
	}

	ctx := context.Background()
	res, err := c.SFClient.CreateGroupSnapshot(ctx, &req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) ListGroupSnapshots(volumes []int64) ([]sdk.GroupSnapshot, error) {
	req := sdk.ListGroupSnapshotsRequest{
		Volumes: volumes,
	}
	ctx := context.Background()
	res, err := c.SFClient.ListGroupSnapshots(ctx, &req)
	if err != nil {
		return nil, err
	}
	// TODO: Handle nil response.GroupSnapshots if necessary, but empty slice is fine
	return res.GroupSnapshots, nil
}

func (c *Client) DeleteGroupSnapshot(groupSnapshotID int64) error {
	req := sdk.DeleteGroupSnapshotRequest{
		GroupSnapshotID: groupSnapshotID,
		SaveMembers:     false,
	}
	ctx := context.Background()
	_, err := c.SFClient.DeleteGroupSnapshot(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetClusterVersion() (string, error) {
	ctx := context.Background()
	res, err := c.SFClient.GetClusterVersionInfo(ctx)
	if err != nil {
		return "", err
	}
	return res.ClusterVersion, nil
}

func (c *Client) ListISCSISessions() ([]sdk.ISCSISession, error) {
	ctx := context.Background()
	res, err := c.SFClient.ListISCSISessions(ctx)
	if err != nil {
		return nil, err
	}
	return res.Sessions, nil
}

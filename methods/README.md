# methods

This provides a simple wrapper around the most basic (and common) operations. This example is geared towards a Kubernetes environment, and takes YAML input as the configuration which specifies the MVIP, SVIP, login credentials and the account name to use for volume operations.

It provides the ability to create, delete, list, update (size or QoS) as well as perform iSCSI connect operations (origianally using the kubernetes-csi iscsi-lib, but solidfire-csi uses Dell's GoiSCSI library).

## Example YAML config

```yaml
endpoint: https://admin:admin@10.1.1.1/json-rpc/12.5
svip: 10.1.2.1:3260
tenantname: tenant1
defaultvolumesize: 1
initiatoriface: default
```

The remaining needed items (like CHAP credentials) should be able to be collected by the init routine itself so long as the endpoint and tenant info is correct.

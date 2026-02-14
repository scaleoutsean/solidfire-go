# solidfire-go

[![Build](https://github.com/scaleoutsean/solidfire-go/actions/workflows/build.yml/badge.svg)](https://github.com/scaleoutsean/solidfire-go/actions/workflows/build.yml)

The solidfire-go library is a Go language SDK for interacting with a SolidFire Cluster.

This particular repository was a derivative of the auto-generated Go SDK package found on NetApp's Ngage BitBucket repository.

What this derivative did is strip out the auto-generated code and just provides the resultant SDK itself. In addition this library also provides a "methods" package with some wrappers around the basic and most common volume operations a user might be interested in.

The methods package wraps soem of the basics into very easy to consume functions and also can serve as a good example on how to use the SDK.

## Build

```sh
go build ./sdk/... ./methods/...
```

Use Go 1.25 or newer. The following dependencies exist.

```sh
github.com/sirupsen/logrus
gopkg.in/yaml.v2
```

Build examples:

```sh
go build -o create-volumes ./example/create-volumes/main.go
go build -o s3-backup ./example/s3-backup/main.go
# or build everything at once
# go build ./sdk/... ./methods/... && go build -o create-volumes example/create-volumes.go && go build -o s3-backup example/s3-backup.go
```

The solidfire-csi repository contains additional examples of using this SDK.

## Compatibility with SolidFire (ElementOS)

solidfire-go is developed and tested with SolidFire 12.5 using SolidFire Demo VM v12.5.0.897.

Older versions are expected to work, but not supported.

Newer versions are supported (using 12.5 or newer JSON-RPC path), but may not support all parameters. One notable exception is the authentication field in iSCSI sessions, enabling access to `chapAlgorithm` in newer SolidFire versions (>=12.7) while maintaining compatibility with 12.5 (and older) that are limited to MD5.

## Contributing

Support for the API methods and parameters from newer versions (12.5+) a goal that depends on contributors.

[What's New](https://docs.netapp.com/us-en/element-software/concepts/concept_rn_whats_new_element.html#garbage-collection-improvement) from the official documentation mentions new features added to API methods in v12.

Community contributions and bug reports are welcome. Please include complete, working request/response examples as the maintainer has no access to SolidFire versions above 12.5.

## Acknowledgements

This repository is an updated fork of John Griffith's repository [https://github.com/j-griffith/solidfire-go](https://github.com/j-griffith/solidfire-go). Thank you, John!

An earlier, auto-generated SDK version from an Ngage Bitbucket repository (which I can't find on the Internet) is copyrighted by NetApp, Inc.


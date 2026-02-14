# Examples

## create-volumes

`GetClusterVersion` is used to check for SolidFire vesion information and depending on it we can use new fields depending on the API version.

```go
    // Example of checking cluster version and new fields
    version, err := clt.GetClusterVersion()
    if err != nil {
        log.Printf("failed to get cluster version: %v\n", err)
    } else {
        log.Printf("Cluster Version: %s\n", version)
    }

    sessions, err := clt.ListISCSISessions()
    if err != nil {
        log.Printf("failed to list iscsi sessions: %v\n", err)
    } else {
        log.Printf("Found %d iSCSI sessions\n", len(sessions))
        for _, s := range sessions {
            if s.Authentication != nil {
                log.Printf("Session %d Authentication Method: %s\n", s.SessionID, s.Authentication.AuthMethod)
                if s.Authentication.ChapAlgorithm != "" {
                    log.Printf("Session %d CHAP Algorithm: %s\n", s.SessionID, s.Authentication.ChapAlgorithm)
                }
            }
        }
    }
```

## s3-backup

This is Go implementation of my old PowerShell "parallel backup to S3" [script](https://github.com/scaleoutsean/awesome-solidfire/blob/master/scripts/parallel-backup-to-s3-v2.ps1) uses a private API call to get the slices.json report to map volumes to nodes for better scheduling and optimal parallelization.

Note that `main()` is a stub. The rest of this example is an exercise for the user.

As the maximum number of bulk job slots is limited, it is recommended to leave some available for restores or other ad-hoc actions without having to interrupt or stop backup to S3.

## Secure API proxy 

See README inside the example directory.
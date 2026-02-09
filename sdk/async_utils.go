package sdk

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

// WaitForAsyncResult polls the GetAsyncResult API until the job completes.
// It returns the final result or an error if the job fails or times out.
func (sfClient *SFClient) WaitForAsyncResult(ctx context.Context, asyncHandle int64) (*GetAsyncResultResult, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			// Prepare request to KeepResult=true so we can read it
			req := &GetAsyncResultRequest{
				AsyncHandle: asyncHandle,
				KeepResult:  true,
			}

			res, sdkErr := sfClient.GetAsyncResult(ctx, req)
			if sdkErr != nil {
				// Treat SDK errors from GetAsyncResult as transient: the async job may not
				// yet be visible via GetAsyncResult even though the cluster returned an
				// async handle from CloneVolume. Log and retry until ctx cancels.
				log.WithContext(ctx).Warnf("GetAsyncResult transient error for handle %d: code=%s detail=%s; will retry", asyncHandle, sdkErr.Code, sdkErr.Detail)
				continue
			}

			if res.Status == "complete" {
				log.WithContext(ctx).Debugf("AsyncHandle %d complete", asyncHandle)
				return res, nil
			}
			if res.Status == "error" {
				return res, fmt.Errorf("async job failed: %v", res.Result)
			}

			log.WithContext(ctx).Debugf("AsyncHandle %d running...", asyncHandle)
		}
	}
}

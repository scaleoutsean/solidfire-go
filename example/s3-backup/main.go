package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/scaleoutsean/solidfire-go/sdk"
)

// Internal structures for parsing slices.json report
type GetReportRequest struct {
	ReportName string `json:"reportName"`
}

type SlicesReport struct {
	Services []ReportService `json:"services"`
	Slices   []ReportSlice   `json:"slices"`
}

type ReportService struct {
	ServiceID int64 `json:"serviceID"`
	NodeID    int64 `json:"nodeID"`
}

type ReportSlice struct {
	VolumeID int64 `json:"volumeID"`
	Primary  int64 `json:"primary"` // ServiceID
}

// ActiveJob tracks a running backup job
type ActiveJob struct {
	VolumeID    int64
	NodeID      int64
	AsyncHandle int64
	StartTime   time.Time
}

func main() {
	// This main function is a stub. In a real scenario, you'd initialize the client
	// and pass a list of volume IDs to RunSmartBackup.
	fmt.Println("This is a library example. Initialize client and call RunSmartBackup.")
}

// RunSmartBackup orchestrates S3 backups respecting per-node limits
func RunSmartBackup(client *sdk.SFClient, volumeIDs []int64, s3Url string) {
	ctx := context.Background()

	// 1. Get Cluster Limits
	limits, err := client.GetLimits(ctx)
	if err != nil {
		log.Fatalf("Failed to get cluster limits: %v", err)
	}

	// Default to 8 if not returned
	maxJobsPerNode := int64(8)
	if limits.BulkVolumeJobsPerNodeMax > 0 {
		maxJobsPerNode = limits.BulkVolumeJobsPerNodeMax
	}

	// Reduce by 1 or 2 to leave room for restores
	if maxJobsPerNode > 2 {
		maxJobsPerNode -= 2
	}

	fmt.Printf("Max concurrent backup jobs per node: %d\n", maxJobsPerNode)

	// Local state tracking
	nodeActiveJobs := make(map[int64]int64)
	activeJobs := make([]*ActiveJob, 0)

	pendingVolumes := make([]int64, len(volumeIDs))
	copy(pendingVolumes, volumeIDs)

	// 2. Get Slice Report to map Volumes -> Nodes
	// This avoids calling GetVolumeStats for every volume in the loop
	volToNode := make(map[int64]int64)
	serviceToNode := make(map[int64]int64)

	log.Println("Fetching slices.json report to map volumes to nodes...")
	var reportRes SlicesReport
	// Use MakeSFCall directly because GetReport generated wrapper might be incomplete
	_, reportErr := client.MakeSFCall(ctx, "GetReport", 1, GetReportRequest{ReportName: "slices.json"}, &reportRes)
	if reportErr != nil {
		log.Printf("Warning: Failed to get slices report, will fall back to GetVolumeStats: %v", reportErr)
	} else {
		// Populate Service -> Node map
		for _, svc := range reportRes.Services {
			serviceToNode[svc.ServiceID] = svc.NodeID
		}
		// Populate Volume -> Node map
		for _, slice := range reportRes.Slices {
			if nodeID, ok := serviceToNode[slice.Primary]; ok {
				volToNode[slice.VolumeID] = nodeID
			}
		}
		log.Printf("Mapped %d volumes to nodes via report.", len(volToNode))
	}

	// Main Loop
	for len(pendingVolumes) > 0 || len(activeJobs) > 0 {

		// A. Check for completed jobs
		finishedIndices := make(map[int]bool)
		for i, job := range activeJobs {
			statusReq := &sdk.GetAsyncResultRequest{
				AsyncHandle: job.AsyncHandle,
			}
			res, err := client.GetAsyncResult(ctx, statusReq)
			if err != nil {
				log.Printf("Error checking job for volume %d: %v", job.VolumeID, err)
				continue
			}

			if res.Status == "complete" {
				fmt.Printf("Backup for Volume %d (Node %d) COMPLETED in %v\n",
					job.VolumeID, job.NodeID, time.Since(job.StartTime))
				finishedIndices[i] = true
				nodeActiveJobs[job.NodeID]--
			} else if res.Status == "error" {
				fmt.Printf("Backup for Volume %d (Node %d) FAILED: %v\n",
					job.VolumeID, job.NodeID, res.Result)
				finishedIndices[i] = true
				nodeActiveJobs[job.NodeID]--
			}
			// If "running", do nothing
		}

		// Remove finished jobs from list
		if len(finishedIndices) > 0 {
			newActive := make([]*ActiveJob, 0)
			for i, job := range activeJobs {
				if !finishedIndices[i] {
					newActive = append(newActive, job)
				}
			}
			activeJobs = newActive
		}

		// B. Try to schedule pending volumes
		remainingPending := make([]int64, 0)
		for _, volID := range pendingVolumes {
			// Find Primary Node for this volume from our map
			primaryNodeID, found := volToNode[volID]
			if !found {
				// Fallback: If not in report (new volume?), try GetVolumeStats
				statsReq := &sdk.GetVolumeStatsRequest{VolumeID: volID}
				stats, err := client.GetVolumeStats(ctx, statsReq)
				if err == nil {
					svcID := stats.VolumeStats.MetadataHosts.Primary
					// Try to resolve service to node again
					if nid, ok := serviceToNode[svcID]; ok {
						primaryNodeID = nid
						volToNode[volID] = nid // cache it
						found = true
					}
				}

				if !found {
					log.Printf("Could not determine node for volume %d, skipping scheduling for now.", volID)
					// Maybe retry getting report later? For now, add back to pending to retry stats later
					remainingPending = append(remainingPending, volID)
					continue
				}
			}

			currentLoad := nodeActiveJobs[primaryNodeID]

			if currentLoad < maxJobsPerNode {
				// START JOB
				fmt.Printf("Starting backup for Volume %d on Node %d (Load: %d/%d)\n",
					volID, primaryNodeID, currentLoad+1, maxJobsPerNode)

				job := startBackupJob(client, volID, s3Url)
				if job != nil {
					job.NodeID = primaryNodeID
					activeJobs = append(activeJobs, job)
					nodeActiveJobs[primaryNodeID]++
				} else {
					// Failed to start, keep in pending? Or drop? Let's keep in pending for retry
					remainingPending = append(remainingPending, volID)
				}
			} else {
				// Node full, keep in pending
				remainingPending = append(remainingPending, volID)
			}
		}
		pendingVolumes = remainingPending

		// Sleep briefly to avoid busy loop
		time.Sleep(5 * time.Second)
	}

	fmt.Println("All backups finished.")
}

func startBackupJob(client *sdk.SFClient, volumeID int64, s3Url string) *ActiveJob {
	req := &sdk.StartBulkVolumeReadRequest{
		VolumeID: volumeID,
		Format:   "native",
		Script:   "backup_to_s3.py",
		ScriptParameters: map[string]interface{}{
			"s3_url": s3Url,
		},
	}

	ctx := context.Background()
	res, err := client.StartBulkVolumeRead(ctx, req)
	if err != nil {
		log.Printf("Failed to start bulk read for volume %d: %v", volumeID, err)
		return nil
	}

	return &ActiveJob{
		VolumeID:    volumeID,
		AsyncHandle: res.AsyncHandle,
		StartTime:   time.Now(),
	}
}

// BackupToS3Simple is the original simple example
func BackupToS3Simple(client *sdk.SFClient, volumeID int64, s3Url string) {
	// 1. Start Bulk Volume Read
	// Script "backup_to_s3.py" is a standard internal script on older Element OS versions.
	// For newer versions or native support, parameters might differ.
	// This illustrates the API call availability.

	req := &sdk.StartBulkVolumeReadRequest{
		VolumeID: volumeID,
		Format:   "native",
		Script:   "backup_to_s3.py", // Or equivalent internal handler
		ScriptParameters: map[string]interface{}{
			"s3_url": s3Url,
		},
	}

	ctx := context.Background()
	res, err := client.StartBulkVolumeRead(ctx, req)
	if err != nil {
		log.Fatalf("Failed to start bulk read: %v", err)
	}

	fmt.Printf("Bulk Volume Read started. Key: %s, URL: %s, AsyncHandle: %d\n", res.Key, res.Url, res.AsyncHandle)

	// 2. Poll for Completion using GetAsyncResult
	asyncHandle := res.AsyncHandle
	for {
		statusReq := &sdk.GetAsyncResultRequest{
			AsyncHandle: asyncHandle,
		}
		statusRes, err := client.GetAsyncResult(ctx, statusReq)
		if err != nil {
			log.Fatalf("Failed to get async result: %v", err)
		}

		fmt.Printf("Job Status: %s\n", statusRes.Status)

		if statusRes.Status == "complete" {
			fmt.Println("Backup complete!")
			break
		}
		if statusRes.Status == "error" {
			log.Fatalf("Backup failed: %v", statusRes.Result)
		}

		time.Sleep(5 * time.Second)
	}
}

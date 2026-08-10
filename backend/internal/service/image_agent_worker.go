package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"sync"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/integration/imageproxy"
)

type ImageDispatcher struct {
	agent     *ImageAgent
	retention time.Duration
}

func NewImageDispatcher(agent *ImageAgent, retention time.Duration) *ImageDispatcher {
	if retention <= 0 {
		retention = 90 * 24 * time.Hour
	}
	return &ImageDispatcher{agent: agent, retention: retention}
}

func (d *ImageDispatcher) Tick(ctx context.Context) error {
	now := time.Now()
	jobs, err := d.agent.repo.ClaimImageJobs(ctx, now, 5*time.Minute, 4)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err = d.process(ctx, job); err != nil {
			slog.Warn("image job processing failed", "job_id", job.ID, "error", err)
		}
	}
	if err = d.agent.repo.CleanupImageJobLogs(ctx, now.Add(-d.retention)); err != nil {
		return err
	}
	_, err = d.agent.repo.CleanupDeletedImageCanvases(ctx, now.Add(-30*24*time.Hour))
	return err
}

func (d *ImageDispatcher) process(ctx context.Context, job domain.ImageJob) error {
	relay, client, err := d.agent.relayClient(ctx, job.RelayID)
	if err != nil {
		return d.finishRetryOrFail(ctx, job, err)
	}
	_ = relay
	model, err := d.agent.repo.GetImageModel(ctx, job.ModelID)
	if err != nil || !model.Enabled {
		if err == nil {
			err = errors.New("image model was disabled before execution")
		}
		return d.finishRetryOrFail(ctx, job, err)
	}
	references, err := d.referenceDataURLs(ctx, job)
	if err != nil {
		return d.finishRetryOrFail(ctx, job, err)
	}
	if job.Kind == "reverse_prompt" {
		description, reverseErr := client.ReversePrompt(ctx, model.ModelID, references)
		if reverseErr != nil {
			return d.finishRetryOrFail(ctx, job, reverseErr)
		}
		job.Status, job.ReversedPrompt, job.Error, job.LockedUntil = "succeeded", description, "", nil
		job.CompletedCount, job.UpdatedAt = 1, time.Now()
		return d.agent.repo.UpdateImageJob(ctx, job)
	}

	pending := make([]domain.ImageJobOutput, 0, len(job.Outputs))
	completed := job.CompletedCount
	for _, output := range job.Outputs {
		if output.Status == "succeeded" {
			if completed == 0 {
				completed++
			}
			continue
		}
		pending = append(pending, output)
	}
	type outputResult struct {
		output domain.ImageJobOutput
		err    error
	}
	results := make(chan outputResult, len(pending))
	semaphore := make(chan struct{}, 2)
	var wg sync.WaitGroup
	for _, output := range pending {
		output := output
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results <- outputResult{output: output, err: ctx.Err()}
				return
			}
			results <- outputResult{output: output, err: d.generateOne(ctx, job, model, client, references, output)}
		}()
	}
	wg.Wait()
	close(results)

	var firstErr error
	failed := 0
	for result := range results {
		if result.err == nil {
			completed++
			continue
		}
		failed++
		if firstErr == nil {
			firstErr = result.err
		}
		if !retryableImageError(result.err) || job.Attempts >= 3 {
			result.output.Status, result.output.Error, result.output.Attempts, result.output.UpdatedAt =
				"failed", result.err.Error(), job.Attempts, time.Now()
			_ = d.agent.repo.UpdateImageJobOutputFailure(ctx, result.output, domain.ImageCanvasNode{
				JobID: job.ID, OutputIndex: result.output.OutputIndex, Error: result.err.Error(), UpdatedAt: time.Now(),
			})
		}
	}
	job.CompletedCount, job.LockedUntil, job.UpdatedAt = completed, nil, time.Now()
	switch {
	case failed == 0:
		job.Status, job.Error = "succeeded", ""
	case completed > 0 && (job.Attempts >= 3 || !retryableImageError(firstErr)):
		job.Status, job.Error = "partial", firstErr.Error()
	case completed == 0 && (job.Attempts >= 3 || !retryableImageError(firstErr)):
		job.Status, job.Error = "failed", firstErr.Error()
	default:
		job.Status, job.Error = "retry", firstErr.Error()
		job.NextAttemptAt = time.Now().Add(backoff(job.Attempts))
	}
	if err = d.agent.repo.UpdateImageJob(ctx, job); err != nil {
		return err
	}
	return firstErr
}

func (d *ImageDispatcher) generateOne(ctx context.Context, job domain.ImageJob, model domain.ImageModel, client *imageproxy.Client, references []string, output domain.ImageJobOutput) error {
	var generated imageproxy.Image
	var err error
	if model.Protocol == "images_generations" {
		generated, err = client.GenerateImage(ctx, model.ModelID, job.Prompt, job.AspectRatio, job.ImageSize)
	} else {
		generated, err = client.GenerateChat(ctx, model.ModelID, job.Prompt, job.AspectRatio, job.ImageSize, references)
	}
	if err != nil {
		return err
	}
	if err = d.agent.scanner.Scan(ctx, generated.Data); err != nil {
		return fmt.Errorf("generated image security scan: %w", err)
	}
	now := time.Now()
	extension := ".png"
	if generated.MIME == "image/jpeg" {
		extension = ".jpg"
	} else if generated.MIME == "image/webp" {
		extension = ".webp"
	}
	width, height := imageDimensions(generated.Data)
	asset := domain.ImageAsset{ID: ids.New("asset"), OwnerID: job.UserID, ProjectID: job.ProjectID,
		ObjectKey: path.Join("image-agent", job.UserID, job.ID, fmt.Sprintf("%d%s", output.OutputIndex, extension)),
		MIMEType:  generated.MIME, FileName: fmt.Sprintf("%s-%d%s", job.ID, output.OutputIndex+1, extension),
		Width: width, Height: height, SizeBytes: int64(len(generated.Data)), Source: "generated", CreatedAt: now}
	if err = d.agent.blobs.Put(ctx, asset.ObjectKey, generated.Data, asset.MIMEType); err != nil {
		return err
	}
	output.Status, output.AssetID, output.Attempts, output.Error, output.UpdatedAt =
		"succeeded", asset.ID, job.Attempts, "", now
	nodeWidth, nodeHeight := nodeDimensions(job.AspectRatio)
	if err = d.agent.repo.SaveImageJobOutput(ctx, output, asset, domain.ImageCanvasNode{
		JobID: job.ID, OutputIndex: output.OutputIndex, Width: nodeWidth, Height: nodeHeight, UpdatedAt: now,
	}); err != nil {
		_ = d.agent.blobs.Delete(ctx, asset.ObjectKey)
		return err
	}
	return nil
}

func (d *ImageDispatcher) referenceDataURLs(ctx context.Context, job domain.ImageJob) ([]string, error) {
	result := make([]string, 0, len(job.ReferenceAssetIDs))
	for _, id := range job.ReferenceAssetIDs {
		asset, err := d.agent.repo.GetImageAsset(ctx, id)
		if err != nil {
			return nil, err
		}
		// A canvas may contain images generated or imported under different
		// projects. CreateJob already validates both the selected target project
		// and every reference asset's source project, so the worker must not
		// require their project IDs to match. The immutable owner relationship is
		// the security boundary that still needs to hold while the job executes.
		if asset.OwnerID != job.UserID {
			return nil, errors.New("reference asset no longer belongs to this user")
		}
		data, err := d.agent.blobs.Get(ctx, asset.ObjectKey)
		if err != nil {
			return nil, err
		}
		result = append(result, "data:"+asset.MIMEType+";base64,"+base64.StdEncoding.EncodeToString(data))
	}
	return result, nil
}

func (d *ImageDispatcher) finishRetryOrFail(ctx context.Context, job domain.ImageJob, cause error) error {
	job.Error, job.LockedUntil, job.UpdatedAt = cause.Error(), nil, time.Now()
	if job.Attempts < 3 && retryableImageError(cause) {
		job.Status, job.NextAttemptAt = "retry", time.Now().Add(backoff(job.Attempts))
	} else {
		job.Status = "failed"
	}
	if err := d.agent.repo.UpdateImageJob(ctx, job); err != nil {
		return err
	}
	return cause
}

func retryableImageError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *imageproxy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Retryable()
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		stringsContainsAny(err.Error(), "timeout", "connection reset", "connection refused", "temporary")
}

func stringsContainsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if len(candidate) > 0 && containsFold(value, candidate) {
			return true
		}
	}
	return false
}

func containsFold(value, candidate string) bool {
	if len(candidate) > len(value) {
		return false
	}
	for index := 0; index+len(candidate) <= len(value); index++ {
		if equalFoldASCII(value[index:index+len(candidate)], candidate) {
			return true
		}
	}
	return false
}

func equalFoldASCII(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		a, b := left[index], right[index]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 3 {
		attempt = 3
	}
	return time.Duration(1<<(attempt-1)) * 5 * time.Second
}

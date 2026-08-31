package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"sync"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/integration/imageproxy"
	"internal-ai-agent/backend/internal/integration/pixian"
)

type ImageDispatcher struct {
	agent     *ImageAgent
	retention time.Duration
}

type imageGenerator interface {
	GenerateChat(context.Context, string, string, string, string, []string) (imageproxy.Image, error)
	GenerateImage(context.Context, string, string, string, string) (imageproxy.Image, error)
	GenerateGPTImage2(context.Context, string, string, string, string, []imageproxy.ReferenceImage) (imageproxy.Image, error)
	GenerateUniversalEdit(context.Context, string, string, string, string, []imageproxy.ReferenceImage) (imageproxy.Image, error)
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
	if err = d.tickBackgroundRemoval(ctx, now); err != nil {
		return err
	}
	if err = d.agent.repo.CleanupImageJobLogs(ctx, now.Add(-d.retention)); err != nil {
		return err
	}
	_, err = d.agent.repo.CleanupDeletedImageCanvases(ctx, now.Add(-30*24*time.Hour))
	return err
}

func (d *ImageDispatcher) tickBackgroundRemoval(ctx context.Context, now time.Time) error {
	config, client, err := d.agent.pixianClient(ctx)
	if err != nil || !config.Enabled {
		return nil
	}
	jobs, err := d.agent.repo.ClaimBackgroundRemovalJobs(ctx, now, time.Duration(config.TimeoutSeconds+60)*time.Second, config.Concurrency)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	for _, value := range jobs {
		job := value
		wg.Add(1)
		go func() {
			defer wg.Done()
			if processErr := d.processBackgroundRemoval(ctx, config, client, job); processErr != nil {
				slog.Warn("background removal job processing failed", "job_id", job.ID, "error", processErr)
			}
		}()
	}
	wg.Wait()
	return nil
}

func (d *ImageDispatcher) processBackgroundRemoval(ctx context.Context, config domain.PixianBackgroundRemovalConfig, client *pixian.Client, job domain.BackgroundRemovalJob) error {
	completed, failed := 0, 0
	charged, calculated := 0.0, 0.0
	var firstErr error
	for _, item := range job.Items {
		if item.Status == "succeeded" {
			completed++
			charged += item.CreditsCharged
			calculated += item.CreditsCalculated
			continue
		}
		if item.Status == "failed" {
			failed++
			continue
		}
		item.Status = "running"
		item.Attempts++
		item.UpdatedAt = time.Now()
		_ = d.agent.repo.UpdateBackgroundRemovalItem(ctx, item, domain.ImageCanvasNode{Status: "pending", UpdatedAt: item.UpdatedAt})
		result, asset, node, err := d.removeBackgroundOne(ctx, config, client, job, item)
		item.CreditsCharged = result.CreditsCharged
		item.CreditsCalculated = result.CreditsCalculated
		item.InputSize = result.InputSize
		item.ResultSize = result.ResultSize
		charged += item.CreditsCharged
		calculated += item.CreditsCalculated
		if err == nil {
			item.Status = "succeeded"
			item.Error = ""
			item.ResultAssetID = asset.ID
			item.UpdatedAt = time.Now()
			if err = d.agent.repo.SaveBackgroundRemovalResult(ctx, item, asset, node); err != nil {
				_ = d.agent.blobs.Delete(ctx, asset.ObjectKey)
			} else {
				completed++
				d.auditBackgroundRemoval(ctx, job, "image.background_removal.item.succeeded", item, map[string]any{"asset_id": asset.ID, "credits_charged": item.CreditsCharged, "credits_calculated": item.CreditsCalculated})
			}
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			item.Error = err.Error()
			item.UpdatedAt = time.Now()
			if shouldRetryPixianError(err, job.Attempts) {
				item.Status = "pending"
				_ = d.agent.repo.UpdateBackgroundRemovalItem(ctx, item, domain.ImageCanvasNode{Status: "pending", Error: "抠图服务暂时不可用，正在重试", UpdatedAt: item.UpdatedAt})
			} else {
				item.Status = "failed"
				failed++
				_ = d.agent.repo.UpdateBackgroundRemovalItem(ctx, item, domain.ImageCanvasNode{Status: "failed", Error: item.Error, UpdatedAt: item.UpdatedAt})
				d.auditBackgroundRemoval(ctx, job, "image.background_removal.item.failed", item, map[string]any{"error": item.Error})
			}
		}
	}
	job.CompletedCount = completed
	job.FailedCount = failed
	job.CreditsCharged = charged
	job.CreditsCalculated = calculated
	job.LockedUntil = nil
	job.UpdatedAt = time.Now()
	message := ""
	if firstErr != nil {
		message = firstErr.Error()
	}
	job.Error = message
	if completed+failed < len(job.Items) {
		job.Status = "retry"
		job.NextAttemptAt = time.Now().Add(time.Duration(min(job.Attempts, 3)*5) * time.Second)
	} else if failed == 0 {
		job.Status = "succeeded"
		job.Error = ""
	} else if completed > 0 {
		job.Status = "partial"
	} else {
		job.Status = "failed"
	}
	if err := d.agent.repo.UpdateBackgroundRemovalJob(ctx, job); err != nil {
		return err
	}
	d.auditBackgroundRemoval(ctx, job, "image.background_removal.job."+job.Status, domain.BackgroundRemovalItem{}, map[string]any{"completed": completed, "failed": failed, "credits_charged": charged, "credits_calculated": calculated})
	return firstErr
}

func (d *ImageDispatcher) removeBackgroundOne(ctx context.Context, config domain.PixianBackgroundRemovalConfig, client *pixian.Client, job domain.BackgroundRemovalJob, item domain.BackgroundRemovalItem) (pixian.Result, domain.ImageAsset, domain.ImageCanvasNode, error) {
	var emptyAsset domain.ImageAsset
	var emptyNode domain.ImageCanvasNode
	source, err := d.agent.repo.GetImageAsset(ctx, item.SourceAssetID)
	if err != nil {
		return pixian.Result{}, emptyAsset, emptyNode, err
	}
	if source.OwnerID != job.UserID || source.ProjectID != item.ProjectID {
		return pixian.Result{}, emptyAsset, emptyNode, errors.New("source image ownership changed")
	}
	data, err := d.agent.blobs.Get(ctx, source.ObjectKey)
	if err != nil {
		return pixian.Result{}, emptyAsset, emptyNode, err
	}
	result, err := client.RemoveBackground(ctx, source.FileName, source.MIMEType, data, job.TestMode, config.MaxPixels)
	if err != nil {
		return result, emptyAsset, emptyNode, err
	}
	if len(result.Data) == 0 || len(result.Data) > 32<<20 {
		return result, emptyAsset, emptyNode, errors.New("Pixian result is empty or exceeds 32 MB")
	}
	if http.DetectContentType(result.Data) != "image/png" {
		return result, emptyAsset, emptyNode, errors.New("Pixian returned invalid PNG data")
	}
	width, height, err := decodeImageDimensions(result.Data)
	if err != nil {
		return result, emptyAsset, emptyNode, fmt.Errorf("decode Pixian PNG: %w", err)
	}
	if err = d.agent.scanner.Scan(ctx, result.Data); err != nil {
		return result, emptyAsset, emptyNode, fmt.Errorf("Pixian result security scan: %w", err)
	}
	now := time.Now()
	asset := domain.ImageAsset{ID: ids.New("asset"), OwnerID: job.UserID, ProjectID: item.ProjectID, ObjectKey: path.Join("image-agent", job.UserID, "background-removal", job.ID, item.ID+".png"), MIMEType: "image/png", FileName: "background-removed-" + item.ID + ".png", Width: width, Height: height, SizeBytes: int64(len(result.Data)), Source: "background_removed", SourceAssetID: source.ID, CreatedAt: now}
	if err = d.agent.blobs.Put(ctx, asset.ObjectKey, result.Data, asset.MIMEType); err != nil {
		return result, emptyAsset, emptyNode, err
	}
	w, h := importedNodeDimensions(width, height)
	node := domain.ImageCanvasNode{ID: item.PlaceholderNodeID, Width: w, Height: h, Status: "ready", UpdatedAt: now}
	return result, asset, node, nil
}

func shouldRetryPixianError(err error, attempt int) bool {
	if attempt >= 3 {
		return false
	}
	var apiErr *pixian.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Retryable()
	}
	return errors.Is(err, context.DeadlineExceeded) || stringsContainsAny(err.Error(), "timeout", "connection reset", "connection refused", "temporary")
}

func (d *ImageDispatcher) auditBackgroundRemoval(ctx context.Context, job domain.BackgroundRemovalJob, action string, item domain.BackgroundRemovalItem, metadata map[string]any) {
	if d.agent.audit == nil {
		return
	}
	actor, err := d.agent.audit.GetUser(ctx, job.UserID)
	if err != nil {
		return
	}
	resourceID := job.ID
	if item.ID != "" {
		resourceID = item.ID
	}
	d.agent.appendAudit(ctx, actor, action, "background_removal_job", resourceID, metadata)
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
	var gptReferences []imageproxy.ReferenceImage
	if model.Protocol == "gpt_image_2" || relay.RelayKey == "xgapi" {
		gptReferences, err = d.referenceImages(ctx, job)
		if err != nil {
			return d.finishRetryOrFail(ctx, job, err)
		}
	}

	pending := make([]domain.ImageJobOutput, 0, len(job.Outputs))
	completed, terminalFailed := 0, 0
	var firstErr error
	for _, output := range job.Outputs {
		switch output.Status {
		case "succeeded":
			completed++
		case "failed":
			terminalFailed++
			if firstErr == nil && output.Error != "" {
				firstErr = errors.New(output.Error)
			}
		default:
			pending = append(pending, output)
		}
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
			results <- outputResult{output: output, err: d.generateOne(ctx, job, model, relay.RelayKey, client, references, gptReferences, output)}
		}()
	}
	wg.Wait()
	close(results)

	retryPending := false
	for result := range results {
		if result.err == nil {
			completed++
			continue
		}
		if firstErr == nil {
			firstErr = result.err
		}
		if shouldRetryImageError(result.err, job.Attempts) {
			retryPending = true
			continue
		}
		terminalFailed++
		result.output.Status, result.output.Error, result.output.Attempts, result.output.UpdatedAt =
			"failed", result.err.Error(), job.Attempts, time.Now()
		_ = d.agent.repo.UpdateImageJobOutputFailure(ctx, result.output, domain.ImageCanvasNode{
			JobID: job.ID, OutputIndex: result.output.OutputIndex, Error: result.err.Error(), UpdatedAt: time.Now(),
		})
	}
	job.CompletedCount, job.LockedUntil, job.UpdatedAt = completed, nil, time.Now()
	failureMessage := "image generation failed"
	if firstErr != nil {
		failureMessage = firstErr.Error()
	}
	switch {
	case retryPending:
		job.Status = "retry"
		job.Error = failureMessage
		job.NextAttemptAt = time.Now().Add(backoff(job.Attempts))
	case terminalFailed == 0:
		job.Status, job.Error = "succeeded", ""
	case completed > 0:
		job.Status, job.Error = "partial", failureMessage
	default:
		job.Status, job.Error = "failed", failureMessage
	}
	if err = d.agent.repo.UpdateImageJob(ctx, job); err != nil {
		return err
	}
	return firstErr
}

func (d *ImageDispatcher) generateOne(ctx context.Context, job domain.ImageJob, model domain.ImageModel, relayKey string, client imageGenerator, references []string, gptReferences []imageproxy.ReferenceImage, output domain.ImageJobOutput) error {
	var generated imageproxy.Image
	var err error
	requestModelID := effectiveImageRequestModelID(model)
	if model.Protocol == "gpt_image_2" {
		if requestedPixels, sizeErr := imageproxy.GPTImagePixelSize(job.AspectRatio, job.ImageSize, gptReferences); sizeErr == nil {
			slog.Info("dispatching GPT Image 2 request", "relay", relayKey, "model", requestModelID,
				"requested_tier", job.ImageSize, "requested_pixels", requestedPixels, "reference_count", len(gptReferences))
		}
		generated, err = client.GenerateGPTImage2(ctx, requestModelID, job.Prompt, job.AspectRatio, job.ImageSize, gptReferences)
	} else if relayKey == "xgapi" && len(gptReferences) > 0 {
		generated, err = client.GenerateUniversalEdit(ctx, requestModelID, job.Prompt, job.AspectRatio, job.ImageSize, gptReferences)
	} else if model.Protocol == "images_generations" {
		generated, err = client.GenerateImage(ctx, requestModelID, job.Prompt, job.AspectRatio, job.ImageSize)
	} else {
		generated, err = client.GenerateChat(ctx, requestModelID, job.Prompt, job.AspectRatio, job.ImageSize, references)
	}
	if err != nil {
		return userFacingImageRelayError(err, relayKey, requestModelID)
	}
	width, height, err := decodeImageDimensions(generated.Data)
	if err != nil {
		return fmt.Errorf("decode generated image dimensions: %w", err)
	}
	if err = d.agent.scanner.Scan(ctx, generated.Data); err != nil {
		return fmt.Errorf("generated image security scan: %w", err)
	}
	now := time.Now()
	output.RequestedSize, output.ActualWidth, output.ActualHeight = job.ImageSize, width, height
	output.ResolutionWarning = gptImageResolutionWarning(model.ModelID, job.AspectRatio, job.ImageSize, width, height)
	if output.ResolutionWarning != "" {
		slog.Warn("image relay returned a lower resolution than requested", "job_id", job.ID,
			"output_index", output.OutputIndex, "requested", job.ImageSize, "width", width, "height", height)
	}
	extension := ".png"
	if generated.MIME == "image/jpeg" {
		extension = ".jpg"
	} else if generated.MIME == "image/webp" {
		extension = ".webp"
	}
	asset := domain.ImageAsset{ID: ids.New("asset"), OwnerID: job.UserID, ProjectID: job.ProjectID,
		ObjectKey: path.Join("image-agent", job.UserID, job.ID, fmt.Sprintf("%d%s", output.OutputIndex, extension)),
		MIMEType:  generated.MIME, FileName: fmt.Sprintf("%s-%d%s", job.ID, output.OutputIndex+1, extension),
		Width: width, Height: height, SizeBytes: int64(len(generated.Data)), Source: "generated", CreatedAt: now}
	if err = d.agent.blobs.Put(ctx, asset.ObjectKey, generated.Data, asset.MIMEType); err != nil {
		return err
	}
	output.Status, output.AssetID, output.Attempts, output.Error, output.UpdatedAt =
		"succeeded", asset.ID, job.Attempts, "", now
	nodeWidth, nodeHeight := importedNodeDimensions(width, height)
	if err = d.agent.repo.SaveImageJobOutput(ctx, output, asset, domain.ImageCanvasNode{
		JobID: job.ID, OutputIndex: output.OutputIndex, Width: nodeWidth, Height: nodeHeight,
		RequestedSize: output.RequestedSize, ActualWidth: width, ActualHeight: height,
		ResolutionWarning: output.ResolutionWarning, UpdatedAt: now,
	}); err != nil {
		_ = d.agent.blobs.Delete(ctx, asset.ObjectKey)
		return err
	}
	return nil
}

func gptImageResolutionWarning(modelID, ratio, tier string, width, height int) string {
	if !isGPTImage2Model(modelID) || tier == "1K" || width <= 0 || height <= 0 {
		return ""
	}
	targets := map[string]map[string][2]int{
		"2K": {
			"1:1": {2048, 2048}, "16:9": {2048, 1152}, "9:16": {1152, 2048},
			"4:3": {2048, 1536}, "3:4": {1536, 2048},
		},
		"4K": {
			"1:1": {2880, 2880}, "16:9": {3840, 2160}, "9:16": {2160, 3840},
			"4:3": {3264, 2448}, "3:4": {2448, 3264},
		},
	}
	target := targets[tier][ratio]
	if ratio == "original" || target == [2]int{} {
		longest := width
		if height > longest {
			longest = height
		}
		minimum := 1843
		if tier == "4K" {
			minimum = 2592
		}
		if longest >= minimum {
			return ""
		}
	} else if float64(width) >= float64(target[0])*0.9 && float64(height) >= float64(target[1])*0.9 {
		return ""
	}
	return fmt.Sprintf("中转站未执行%s档位，已保留实际%d×%d结果", tier, width, height)
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

func (d *ImageDispatcher) referenceImages(ctx context.Context, job domain.ImageJob) ([]imageproxy.ReferenceImage, error) {
	result := make([]imageproxy.ReferenceImage, 0, len(job.ReferenceAssetIDs))
	for _, id := range job.ReferenceAssetIDs {
		asset, err := d.agent.repo.GetImageAsset(ctx, id)
		if err != nil {
			return nil, err
		}
		if asset.OwnerID != job.UserID {
			return nil, errors.New("reference asset no longer belongs to this user")
		}
		data, err := d.agent.blobs.Get(ctx, asset.ObjectKey)
		if err != nil {
			return nil, err
		}
		width, height := asset.Width, asset.Height
		if width <= 0 || height <= 0 {
			width, height, err = decodeImageDimensions(data)
			if err != nil {
				return nil, fmt.Errorf("decode reference image dimensions: %w", err)
			}
		}
		result = append(result, imageproxy.ReferenceImage{FileName: asset.FileName, MIME: asset.MIMEType, Data: data, Width: width, Height: height})
	}
	return result, nil
}

func (d *ImageDispatcher) finishRetryOrFail(ctx context.Context, job domain.ImageJob, cause error) error {
	job.Error, job.LockedUntil, job.UpdatedAt = cause.Error(), nil, time.Now()
	if shouldRetryImageError(cause, job.Attempts) {
		job.Status, job.NextAttemptAt = "retry", time.Now().Add(backoff(job.Attempts))
	} else {
		job.Status = "failed"
	}
	if err := d.agent.repo.UpdateImageJob(ctx, job); err != nil {
		return err
	}
	return cause
}

func shouldRetryImageError(err error, attempt int) bool {
	return attempt < 3 && retryableImageError(err)
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

func userFacingImageRelayError(err error, relayKey, modelID string) error {
	var apiErr *imageproxy.APIError
	if errors.As(err, &apiErr) && stringsContainsAny(apiErr.Body, `"code":"model_not_found"`, "no available channel for model") {
		slog.Warn("image relay model channel is unavailable", "relay", relayKey, "model", modelID,
			"status", apiErr.StatusCode, "error", apiErr.Body)
		return errors.New("中转站当前没有该模型的可用通道，请同步模型或更换已启用模型")
	}
	return err
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

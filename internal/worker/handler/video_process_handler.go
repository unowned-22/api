package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	mediavideo "github.com/unowned-22/api/internal/domain/media/video"
	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	domainvideo "github.com/unowned-22/api/internal/domain/video"
	"github.com/unowned-22/api/internal/logger"
)

type VideoProcessHandler struct {
	videoRepo domainvideo.Repository
	storage   domainstorage.Storage
	bucket    string
	publisher event.Publisher
}

func NewVideoProcessHandler(videoRepo domainvideo.Repository, storage domainstorage.Storage, bucket string, publisher event.Publisher) *VideoProcessHandler {
	return &VideoProcessHandler{videoRepo: videoRepo, storage: storage, bucket: bucket, publisher: publisher}
}

func (h *VideoProcessHandler) EventName() event.Name { return event.Name("video.process") }

// videoProgressPayload is the JSON body for event.VideoProcessingProgress.
type videoProgressPayload struct {
	VideoID    int64  `json:"video_id"`
	OwnerID    int64  `json:"owner_id"`
	Stage      string `json:"stage"`
	Percent    int    `json:"percent"`
	ETASeconds int    `json:"eta_seconds"` // -1 if unknown
}

// progressTracker throttles DB writes and AMQP publishes to at most once per
// second per video, and only when percent has advanced by at least 1.
type progressTracker struct {
	videoID   int64
	repo      domainvideo.Repository
	publisher event.Publisher
	ownerID   int64
	startedAt time.Time

	mu          sync.Mutex
	lastPercent int
	lastPublish time.Time
}

func (t *progressTracker) update(ctx context.Context, stage string, percent int) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(t.lastPublish)
	if elapsed < time.Second && percent-t.lastPercent < 1 {
		return
	}

	t.lastPercent = percent
	t.lastPublish = now

	// Best-effort DB write — log warn on failure, never fail processing.
	if err := t.repo.UpdateProgress(ctx, t.videoID, stage, percent); err != nil {
		logger.Log.WithError(err).WithField("video_id", t.videoID).Warn("video process: UpdateProgress failed")
	}

	// Compute linear ETA.
	etaSec := -1
	totalElapsed := now.Sub(t.startedAt).Seconds()
	if percent > 0 {
		etaSec = int(totalElapsed / float64(percent) * float64(100-percent))
	}

	p := videoProgressPayload{
		VideoID:    t.videoID,
		OwnerID:    t.ownerID,
		Stage:      stage,
		Percent:    percent,
		ETASeconds: etaSec,
	}
	payloadBytes, _ := json.Marshal(p)
	if err := t.publisher.Publish(ctx, event.Event{Name: event.VideoProcessingProgress, Payload: payloadBytes}); err != nil {
		logger.Log.WithError(err).WithField("video_id", t.videoID).Warn("video process: publish progress event failed")
	}
}

func (h *VideoProcessHandler) Handle(ctx context.Context, payload []byte) error {
	var job mediavideo.ProcessJob
	if err := json.Unmarshal(payload, &job); err != nil {
		logger.Log.WithError(err).Error("video process: invalid payload")
		return nil
	}

	videoRow, err := h.videoRepo.GetByID(ctx, job.VideoID)
	if err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: failed to load video")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}

	if err := h.videoRepo.MarkProcessing(ctx, job.VideoID); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: failed to mark processing")
		return nil
	}

	tracker := &progressTracker{
		videoID:   job.VideoID,
		repo:      h.videoRepo,
		publisher: h.publisher,
		ownerID:   videoRow.UserID,
		startedAt: time.Now(),
	}

	tmpDir, err := os.MkdirTemp("", "vproc-*")
	if err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: tmpdir failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}
	defer os.RemoveAll(tmpDir)

	// — Stage: downloading (0→5%) —
	tracker.update(ctx, "downloading", 0)
	rawPath := filepath.Join(tmpDir, "raw")
	if err := h.downloadObject(ctx, job.RawKey, rawPath); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: download failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}
	tracker.update(ctx, "downloading", 5)

	// — Stage: probing (5→7%) —
	tracker.update(ctx, "probing", 5)
	meta, err := mediavideo.Probe(ctx, rawPath)
	if err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: probe failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}
	tracker.update(ctx, "probing", 7)

	// — Stage: thumbnail (7→10%) —
	tracker.update(ctx, "thumbnail", 7)
	thumbPath := filepath.Join(tmpDir, "thumb.jpg")
	thumbAt := "00:00:05"
	if meta.Duration > 0 && meta.Duration < 5 {
		thumbAt = "00:00:01"
	}
	if err := runFFmpeg(ctx, "ffmpeg", "-y", "-ss", thumbAt, "-i", rawPath, "-frames:v", "1", thumbPath); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: thumbnail failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}
	tracker.update(ctx, "thumbnail", 10)

	// — Stage: transcode variants (10→90%) —
	variants := variantsForHeight(meta.Height)
	baseKey := fmt.Sprintf("videos/%d/hls/%d", videoRow.ChannelID, job.VideoID)

	// Distribute the 80% transcode budget across variants proportionally.
	transcodeStart := 10
	transcodeBudget := 80
	perVariant := transcodeBudget / len(variants)

	stageNames := map[string]string{
		"360p":  "transcode_360p",
		"720p":  "transcode_720p",
		"1080p": "transcode_1080p",
	}

	for i, variant := range variants {
		variantDir := filepath.Join(tmpDir, variant.label)
		if err := os.MkdirAll(variantDir, 0o755); err != nil {
			logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: create variant dir failed")
			_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
			return nil
		}
		playlistPath := filepath.Join(variantDir, "playlist.m3u8")
		stageName := stageNames[variant.label]
		variantOffset := transcodeStart + i*perVariant

		tracker.update(ctx, stageName, variantOffset)

		onProgress := func(fraction float64) {
			pct := variantOffset + int(fraction*float64(perVariant))
			tracker.update(ctx, stageName, pct)
		}

		if err := runFFmpegWithProgress(ctx, meta.Duration, onProgress, "ffmpeg",
			"-y", "-i", rawPath,
			"-vf", fmt.Sprintf("scale=-2:%d", variant.height),
			"-c:v", "libx264", "-crf", "23", "-preset", "fast",
			"-c:a", "aac", "-b:a", "128k",
			"-hls_time", "6", "-hls_playlist_type", "vod",
			"-hls_segment_filename", filepath.Join(variantDir, "seg_%03d.ts"),
			playlistPath,
		); err != nil {
			logger.Log.WithError(err).WithField("video_id", job.VideoID).WithField("variant", variant.label).Error("video process: hls transcode failed")
			_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
			return nil
		}

		tracker.update(ctx, stageName, variantOffset+perVariant)
	}

	masterPlaylistPath := filepath.Join(tmpDir, "master.m3u8")
	if err := writeMasterPlaylist(masterPlaylistPath, variants, meta.Width, meta.Height); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: write master playlist failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}

	// — Stage: uploading (90→100%) —
	tracker.update(ctx, "uploading", 90)

	for _, variant := range variants {
		variantDir := filepath.Join(tmpDir, variant.label)
		entries, err := os.ReadDir(variantDir)
		if err != nil {
			logger.Log.WithError(err).WithField("video_id", job.VideoID).WithField("variant", variant.label).Error("video process: read variant dir failed")
			_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
			return nil
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			localPath := filepath.Join(variantDir, entry.Name())
			key := fmt.Sprintf("%s/%s/%s", baseKey, variant.label, entry.Name())
			contentType := "video/mp2t"
			if strings.HasSuffix(entry.Name(), ".m3u8") {
				contentType = "application/x-mpegURL"
			}
			if err := h.uploadFile(ctx, key, localPath, contentType); err != nil {
				logger.Log.WithError(err).WithField("video_id", job.VideoID).WithField("variant", variant.label).Error("video process: upload hls file failed")
				_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
				return nil
			}
		}
	}

	masterKey := fmt.Sprintf("%s/master.m3u8", baseKey)
	if err := h.uploadFile(ctx, masterKey, masterPlaylistPath, "application/x-mpegURL"); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: upload master playlist failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}

	thumbKey := fmt.Sprintf("videos/%d/thumb/%d.jpg", videoRow.ChannelID, job.VideoID)
	if err := h.uploadFile(ctx, thumbKey, thumbPath, "image/jpeg"); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: upload thumbnail failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}

	tracker.update(ctx, "uploading", 100)

	if err := h.videoRepo.ReadyForPublish(ctx, job.VideoID, masterKey, "", "", thumbKey, meta.Duration, meta.Width, meta.Height, meta.Size, meta.VideoCodec, meta.AudioCodec); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: mark ready failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}

	payloadOut, _ := json.Marshal(map[string]any{
		"video_id":      job.VideoID,
		"channel_id":    videoRow.ChannelID,
		"channel_name":  "",
		"title":         videoRow.Title,
		"thumbnail_key": thumbKey,
	})
	if err := h.publisher.Publish(ctx, event.Event{Name: event.VideoPublished, Payload: payloadOut}); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Warn("video process: publish event failed")
	}
	return nil
}

type hlsVariant struct {
	label  string
	height int
}

func variantsForHeight(h int) []hlsVariant {
	all := []hlsVariant{{label: "360p", height: 360}, {label: "720p", height: 720}, {label: "1080p", height: 1080}}
	if h < 480 {
		return []hlsVariant{{label: "360p", height: 360}}
	}
	if h < 1080 {
		return []hlsVariant{{label: "360p", height: 360}, {label: "720p", height: 720}}
	}
	return all
}

func writeMasterPlaylist(path string, variants []hlsVariant, sourceWidth, sourceHeight int) error {
	var width int
	var height int
	if sourceWidth > 0 && sourceHeight > 0 {
		width = int(float64(sourceWidth) / float64(sourceHeight) * float64(360))
		if width%2 != 0 {
			width++
		}
		height = 360
	} else {
		width = 640
		height = 360
	}
	lines := []string{"#EXTM3U", "#EXT-X-VERSION:3"}
	for _, variant := range variants {
		bandwidth := 800000
		if variant.height >= 720 {
			bandwidth = 2800000
		}
		if variant.height >= 1080 {
			bandwidth = 5000000
		}
		resWidth := width
		resHeight := height
		if variant.height == 720 {
			resWidth = 1280
			resHeight = 720
		} else if variant.height == 1080 {
			resWidth = 1920
			resHeight = 1080
		}
		if sourceWidth > 0 && sourceHeight > 0 {
			aspectRatio := float64(sourceWidth) / float64(sourceHeight)
			resWidth = int(math.Round(float64(variant.height)*aspectRatio/2)) * 2
			if resWidth < 2 {
				resWidth = 2
			}
			resHeight = variant.height
		}
		lines = append(lines, fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d", bandwidth, resWidth, resHeight))
		lines = append(lines, fmt.Sprintf("%s/playlist.m3u8", variant.label))
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func (h *VideoProcessHandler) downloadObject(ctx context.Context, key, dst string) error {
	info, err := h.storage.StatObject(ctx, h.bucket, key)
	if err != nil {
		return err
	}
	if info == nil {
		return fmt.Errorf("missing object info")
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	r, err := h.storage.GetObject(ctx, h.bucket, key)
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(f, r)
	return err
}

func (h *VideoProcessHandler) uploadFile(ctx context.Context, key, path, contentType string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	_, err = h.storage.PutObject(ctx, h.bucket, key, f, info.Size(), contentType)
	return err
}

// runFFmpeg runs ffmpeg synchronously (for quick operations like thumbnail extraction).
func runFFmpeg(ctx context.Context, name string, args ...string) error {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w: %s", name, err, string(out))
	}
	return nil
}

// runFFmpegWithProgress runs ffmpeg with -progress pipe:1, parses out_time_ms lines and
// calls onProgress(fraction float64) for each reported point (fraction in [0,1], clamped).
// onProgress is called from the same goroutine that reads the pipe — callers must not block
// for long inside it (the tracker's throttle ensures this).
func runFFmpegWithProgress(ctx context.Context, totalDurationSec float64, onProgress func(fraction float64), name string, args ...string) error {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// -progress and -nostats are global ffmpeg options; they must come before any
	// input/output specs. Prepend them at the start of the argument list.
	fullArgs := make([]string, 0, len(args)+3)
	fullArgs = append(fullArgs, "-progress", "pipe:1", "-nostats")
	fullArgs = append(fullArgs, args...)

	cmd := exec.CommandContext(cctx, name, fullArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("%s: stdout pipe: %w", name, err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: start: %w", name, err)
	}

	// Parse progress from stdout in the same goroutine.
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "out_time_ms=") {
			val := strings.TrimPrefix(line, "out_time_ms=")
			ms, err := strconv.ParseInt(val, 10, 64)
			if err != nil || totalDurationSec <= 0 {
				continue
			}
			fraction := float64(ms) / 1e6 / totalDurationSec
			if fraction < 0 {
				fraction = 0
			}
			if fraction > 1 {
				fraction = 1
			}
			onProgress(fraction)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s failed: %w: %s", name, err, stderrBuf.String())
	}
	return nil
}

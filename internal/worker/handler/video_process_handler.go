package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

	tmpDir, err := os.MkdirTemp("", "vproc-*")
	if err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: tmpdir failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}
	defer os.RemoveAll(tmpDir)

	rawPath := filepath.Join(tmpDir, "raw")
	if err := h.downloadObject(ctx, job.RawKey, rawPath); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: download failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}

	meta, err := mediavideo.Probe(ctx, rawPath)
	if err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: probe failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}

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

	mp4360Path := filepath.Join(tmpDir, "360p.mp4")
	if err := runFFmpeg(ctx, "ffmpeg", "-y", "-i", rawPath, "-vf", "scale=-2:360", "-c:v", "libx264", "-crf", "23", "-preset", "fast", "-c:a", "aac", "-b:a", "128k", mp4360Path); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: 360p transcode failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}

	var mp4720Path string
	if meta.Height >= 720 {
		mp4720Path = filepath.Join(tmpDir, "720p.mp4")
		if err := runFFmpeg(ctx, "ffmpeg", "-y", "-i", rawPath, "-vf", "scale=-2:720", "-c:v", "libx264", "-crf", "23", "-preset", "fast", "-c:a", "aac", "-b:a", "128k", mp4720Path); err != nil {
			logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: 720p transcode failed")
			_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
			return nil
		}
	}

	mp4360Key := fmt.Sprintf("videos/%d/mp4_360/%d.mp4", videoRow.ChannelID, job.VideoID)
	if err := h.uploadFile(ctx, mp4360Key, mp4360Path, "video/mp4"); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: upload 360p failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}

	var mp4720Key string
	if mp4720Path != "" {
		mp4720Key = fmt.Sprintf("videos/%d/mp4_720/%d.mp4", videoRow.ChannelID, job.VideoID)
		if err := h.uploadFile(ctx, mp4720Key, mp4720Path, "video/mp4"); err != nil {
			logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: upload 720p failed")
			_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
			return nil
		}
	}

	thumbKey := fmt.Sprintf("videos/%d/thumb/%d.jpg", videoRow.ChannelID, job.VideoID)
	if err := h.uploadFile(ctx, thumbKey, thumbPath, "image/jpeg"); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: upload thumbnail failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}

	if err := h.videoRepo.ReadyForPublish(ctx, job.VideoID, "", mp4360Key, mp4720Key, thumbKey, meta.Duration, meta.Width, meta.Height, meta.Size, meta.VideoCodec, meta.AudioCodec); err != nil {
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

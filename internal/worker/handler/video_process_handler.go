package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	variants := variantsForHeight(meta.Height)
	baseKey := fmt.Sprintf("videos/%d/hls/%d", videoRow.ChannelID, job.VideoID)
	for _, variant := range variants {
		variantDir := filepath.Join(tmpDir, variant.label)
		if err := os.MkdirAll(variantDir, 0o755); err != nil {
			logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: create variant dir failed")
			_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
			return nil
		}
		playlistPath := filepath.Join(variantDir, "playlist.m3u8")
		if err := runFFmpeg(ctx, "ffmpeg", "-y", "-i", rawPath, "-vf", fmt.Sprintf("scale=-2:%d", variant.height), "-c:v", "libx264", "-crf", "23", "-preset", "fast", "-c:a", "aac", "-b:a", "128k", "-hls_time", "6", "-hls_playlist_type", "vod", "-hls_segment_filename", filepath.Join(variantDir, "seg_%03d.ts"), playlistPath); err != nil {
			logger.Log.WithError(err).WithField("video_id", job.VideoID).WithField("variant", variant.label).Error("video process: hls transcode failed")
			_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
			return nil
		}
	}

	masterPlaylistPath := filepath.Join(tmpDir, "master.m3u8")
	if err := writeMasterPlaylist(masterPlaylistPath, variants, meta.Width, meta.Height); err != nil {
		logger.Log.WithError(err).WithField("video_id", job.VideoID).Error("video process: write master playlist failed")
		_ = h.videoRepo.MarkFailed(ctx, job.VideoID)
		return nil
	}

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

package video

import (
	"context"

	"github.com/unowned-22/api/internal/domain/media"
)

// ProcessJob describes a pending video-transcode operation. It is
// enqueued after a raw video has been uploaded to object storage and
// is consumed by the async worker (see README.md for the full flow).
type ProcessJob struct {
	// UserID is the uploader.
	UserID int64

	VideoID int64

	// CommunityID replaced the old ChannelID after Stage 2 migration.
	// The field name in the JSON envelope is kept as "community_id" so
	// the worker log messages stay consistent with the new schema name.
	CommunityID int64 `json:"community_id"`

	// RawKey is the object-storage key of the already-uploaded raw video.
	RawKey string

	// Variants describes the output variants the worker should produce.
	Variants []media.VariantSpec
}

// JobQueue is implemented by the concrete queue adapter (RabbitMQ, etc.).
type JobQueue interface {
	Enqueue(ctx context.Context, job ProcessJob) error
}

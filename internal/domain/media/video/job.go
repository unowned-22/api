package video

import (
	"context"

	"github.com/unowned-22/api/internal/domain/media"
)

// ProcessJob describes a pending video-transcode operation.  It is
// enqueued after a raw video has been uploaded to object storage and
// is consumed by an async worker (see README.md for the full flow).
type ProcessJob struct {
	// UserID is the owner of the media.
	UserID int64

	// RawKey is the object-storage key of the already-uploaded raw video.
	// The worker fetches this key, probes it, transcodes it to Variants,
	// extracts a thumbnail frame, and updates the DB record.
	RawKey string

	// Variants describes the output variants the worker should produce.
	Variants []media.VariantSpec
}

// JobQueue is implemented by the concrete queue adapter (RabbitMQ, Redis
// Streams, etc.).  No implementation is provided in this task — see the
// existing async worker pattern used for bucket provisioning
// (errs.ErrUserStorageNotProvisioned) as prior art for the implementation.
type JobQueue interface {
	Enqueue(ctx context.Context, job ProcessJob) error
}

package queue

import (
	"context"
	"encoding/json"

	domainevent "github.com/unowned-22/api/internal/domain/event"
	domainvideo "github.com/unowned-22/api/internal/domain/media/video"
)

type VideoJobQueue struct {
	pub   *AMQPPublisher
	queue string
}

func NewVideoJobQueue(pub *AMQPPublisher, queue string) *VideoJobQueue {
	return &VideoJobQueue{pub: pub, queue: queue}
}

func (q *VideoJobQueue) Enqueue(ctx context.Context, job domainvideo.ProcessJob) error {
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.pub.Publish(ctx, domainevent.Event{Name: domainevent.Name(q.queue), Payload: b})
}

var _ domainvideo.JobQueue = (*VideoJobQueue)(nil)

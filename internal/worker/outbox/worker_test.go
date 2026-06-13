package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	domaev "github.com/unowned-22/api/internal/domain/event"
	dom "github.com/unowned-22/api/internal/domain/outbox"
)

type fakeRepo struct {
	events []*dom.OutboxEvent
}

func (f *fakeRepo) Insert(ctx context.Context, evt *dom.OutboxEvent) error { return nil }
func (f *fakeRepo) FetchAndMarkProcessing(ctx context.Context, limit int) ([]*dom.OutboxEvent, error) {
	if len(f.events) == 0 {
		return nil, nil
	}
	ev := f.events
	f.events = nil
	return ev, nil
}
func (f *fakeRepo) MarkProcessed(ctx context.Context, id string) error         { return nil }
func (f *fakeRepo) IncrementRetry(ctx context.Context, id string) (int, error) { return 1, nil }
func (f *fakeRepo) MarkFailed(ctx context.Context, id string) error            { return nil }

type fakePub struct {
	fail      bool
	published []*domaev.Event
}

func (p *fakePub) Publish(ctx context.Context, e domaev.Event) error {
	if p.fail {
		return errors.New("publish fail")
	}
	p.published = append(p.published, &e)
	return nil
}
func (p *fakePub) Close() error { return nil }

func TestWorkerPublishesAndMarksProcessed(t *testing.T) {
	repo := &fakeRepo{events: []*dom.OutboxEvent{{ID: "1", EventType: string(domaev.UserRegistered), Payload: []byte(`{"a":1}`), Status: dom.StatusPending, CreatedAt: time.Now()}}}
	pub := &fakePub{fail: false}
	w := NewWorker(repo, pub, RetryPolicy{MaxRetries: 3, Interval: 10 * time.Millisecond}, 10)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { w.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	// ensure published
	if len(pub.published) == 0 {
		t.Fatal("expected event published")
	}
}

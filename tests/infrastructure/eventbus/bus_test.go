package eventbus

import (
	"context"
	"errors"
	"testing"
	"time"

	dom "github.com/unowned-22/api/internal/domain/eventbus"
	"github.com/unowned-22/api/internal/infrastructure/eventbus"
	"github.com/unowned-22/api/internal/logger"
)

type testEvent struct{}

func (t testEvent) EventName() string { return "user.registered" }

type funcHandler func(ctx context.Context, event dom.Event) error

func (f funcHandler) Handle(ctx context.Context, event dom.Event) error { return f(ctx, event) }

func TestMultipleSubscribersReceiveEvent(t *testing.T) {
	// ensure logger is initialized for structured logs used by the bus
	_ = logger.Init()
	bus := inmemory.NewInMemoryBus()

	ch1 := make(chan struct{}, 1)
	ch2 := make(chan struct{}, 1)

	bus.Subscribe("user.registered", funcHandler(func(ctx context.Context, event dom.Event) error {
		ch1 <- struct{}{}
		return nil
	}))

	bus.Subscribe("user.registered", funcHandler(func(ctx context.Context, event dom.Event) error {
		ch2 <- struct{}{}
		return nil
	}))

	if err := bus.Publish(context.Background(), testEvent{}); err != nil {
		t.Fatalf("publish error: %v", err)
	}

	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("handler 1 did not receive event")
	}

	select {
	case <-ch2:
	case <-time.After(time.Second):
		t.Fatal("handler 2 did not receive event")
	}
}

func TestHandlerIsolationAndFailure(t *testing.T) {
	bus := inmemory.NewInMemoryBus()

	okCh := make(chan struct{}, 1)
	errCh := make(chan struct{}, 1)

	// Normal handler
	bus.Subscribe("user.registered", funcHandler(func(ctx context.Context, event dom.Event) error {
		okCh <- struct{}{}
		return nil
	}))

	// Erroring handler
	bus.Subscribe("user.registered", funcHandler(func(ctx context.Context, event dom.Event) error {
		errCh <- struct{}{}
		return errors.New("handler failed")
	}))

	// Panicking handler (should be recovered)
	bus.Subscribe("user.registered", funcHandler(func(ctx context.Context, event dom.Event) error {
		panic("oh no")
	}))

	if err := bus.Publish(context.Background(), testEvent{}); err != nil {
		t.Fatalf("publish error: %v", err)
	}

	// Ensure normal handler got it
	select {
	case <-okCh:
	case <-time.After(time.Second):
		t.Fatal("normal handler did not receive event")
	}

	// Ensure erroring handler got it (even though it returned error)
	select {
	case <-errCh:
	case <-time.After(time.Second):
		t.Fatal("erroring handler did not receive event")
	}
}

func TestPublishIsNonBlocking(t *testing.T) {
	bus := inmemory.NewInMemoryBus()

	block := make(chan struct{})
	bus.Subscribe("user.registered", funcHandler(func(ctx context.Context, event dom.Event) error {
		<-block
		return nil
	}))

	start := time.Now()
	if err := bus.Publish(context.Background(), testEvent{}); err != nil {
		t.Fatalf("publish error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Fatalf("publish took too long: %v", elapsed)
	}

	// unblock the handler and allow goroutine to exit
	close(block)
}

func TestContextAwareness(t *testing.T) {
	bus := inmemory.NewInMemoryBus()

	gotCanceled := make(chan bool, 1)

	bus.Subscribe("user.registered", funcHandler(func(ctx context.Context, event dom.Event) error {
		gotCanceled <- (ctx.Err() != nil)
		return nil
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := bus.Publish(ctx, testEvent{}); err != nil {
		t.Fatalf("publish error: %v", err)
	}

	select {
	case canceled := <-gotCanceled:
		if !canceled {
			t.Fatal("handler did not observe canceled context")
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not run")
	}
}

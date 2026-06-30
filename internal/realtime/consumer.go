package realtime

import (
	"context"
	"fmt"
	"time"

	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/domain/story"
	"github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	"github.com/unowned-22/api/internal/logger"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

type Consumer struct {
	consumer *queue.AMQPConsumer
}

// NewConsumer wires all realtime event handlers.
// videosubscription.Repository is replaced by community.Repository (Stage 2).
func NewConsumer(
	cfg *config.Config,
	friendshipRepo friendship.Repository,
	storyRepo story.StoryRepository,
	userSettingsRepo usersettings.Repository,
	notificationRepo notification.Repository,
	hub *ws.Hub,
	messengerMemberRepo messenger.MemberRepository,
	communityRepo community.Repository, // was: videosubscription.Repository
) (*Consumer, error) {
	handlers := map[event.Name]event.Handler{
		event.FriendRequestReceived:    NewFriendRequestReceivedHandler(userSettingsRepo, notificationRepo, hub),
		event.FriendRequestAccepted:    NewFriendRequestAcceptedHandler(userSettingsRepo, notificationRepo, hub),
		event.StoryPublished:           NewStoryPublishedHandler(friendshipRepo, storyRepo, userSettingsRepo, notificationRepo, hub),
		event.PhotoLiked:               NewPhotoLikedHandler(userSettingsRepo, notificationRepo, hub),
		event.PhotoCommented:           NewPhotoCommentedHandler(userSettingsRepo, notificationRepo, hub),
		event.CommentReplied:           NewCommentRepliedHandler(userSettingsRepo, notificationRepo, hub),
		event.CommentLiked:             NewCommentLikedHandler(userSettingsRepo, notificationRepo, hub),
		event.VideoPublished:           NewVideoPublishedHandler(communityRepo, hub),
		event.VideoProcessingProgress:  NewVideoProcessingProgressHandler(hub),
		event.MessengerMessageSent:     NewMessengerMessageSentHandler(hub, userSettingsRepo, notificationRepo),
		event.MessengerScheduledReady:  NewMessengerScheduledReadyHandler(hub),
		event.MessengerReactionAdded:   NewMessengerReactionHandler(event.MessengerReactionAdded, messengerMemberRepo, hub),
		event.MessengerReactionRemoved: NewMessengerReactionHandler(event.MessengerReactionRemoved, messengerMemberRepo, hub),
		event.MessengerDeliveryUpdated: NewMessengerDeliveryUpdatedHandler(hub),
		event.MessengerMessagePinned:   NewMessengerPinHandler(event.MessengerMessagePinned, messengerMemberRepo, hub),
		event.MessengerMessageUnpinned: NewMessengerPinHandler(event.MessengerMessageUnpinned, messengerMemberRepo, hub),
		event.MessengerMessageEdited:   NewMessengerMessageEditedHandler(messengerMemberRepo, hub),
		event.MessengerMessageDeleted:  NewMessengerMessageDeletedHandler(messengerMemberRepo, hub),
		event.MessengerReadReceipt:     NewMessengerReadReceiptHandler(messengerMemberRepo, hub),
		event.MessengerMemberAdded:     NewMessengerMemberAddedHandler(messengerMemberRepo, hub),
		event.MessengerMemberRemoved:   NewMessengerMemberRemovedHandler(messengerMemberRepo, hub),
		event.MessengerTyping:          NewMessengerTypingHandler(messengerMemberRepo, hub),
		event.CommunityPostPublished:   NewCommunityPostHandler(communityRepo, hub),
	}

	consumer, err := queue.NewConsumer(queue.ConsumerConfig{
		URL:                  cfg.RabbitMQURL,
		Exchange:             cfg.RabbitMQExchange,
		Queue:                cfg.RabbitMQRealtimeQueue,
		Tag:                  "serve-realtime",
		DeadLetterExchange:   cfg.RabbitMQDeadLetterExchange,
		DeadLetterRoutingKey: cfg.RabbitMQRealtimeDeadLetterRoutingKey,
	}, handlers)
	if err != nil {
		return nil, fmt.Errorf("failed to create realtime AMQP consumer: %w", err)
	}

	return &Consumer{consumer: consumer}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	logger.Log.Info("Realtime consumer started")
	if err := c.consumer.Consume(); err != nil {
		return fmt.Errorf("failed to start realtime consumer: %w", err)
	}

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.consumer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown realtime consumer: %w", err)
	}

	return nil
}

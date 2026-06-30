DROP TABLE IF EXISTS bot_accounts;
DROP TABLE IF EXISTS user_presence;
DROP TABLE IF EXISTS messenger_blocked_users;
DROP TABLE IF EXISTS messenger_privacy_settings;
DROP TABLE IF EXISTS message_drafts;
DROP TABLE IF EXISTS message_delivery_status;
DROP TABLE IF EXISTS message_reactions;
DROP TABLE IF EXISTS message_attachments;

ALTER TABLE conversations DROP CONSTRAINT IF EXISTS fk_conversations_last_message;

DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversation_members;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS community_members;
DROP TABLE IF EXISTS communities;
DROP TABLE IF EXISTS video_playlist_items;
DROP TABLE IF EXISTS video_playlists;
DROP TABLE IF EXISTS video_comment_likes;
DROP TABLE IF EXISTS video_comments;
DROP TABLE IF EXISTS video_likes;
DROP TABLE IF EXISTS video_views;
DROP TABLE IF EXISTS video_tags;
DROP TABLE IF EXISTS videos;
DROP VIEW IF EXISTS feed_items;
DROP TABLE IF EXISTS community_posts;
DROP TABLE IF EXISTS user_posts;

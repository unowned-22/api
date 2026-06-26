ALTER TABLE users DROP COLUMN IF EXISTS is_bot;

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

-- link existing refresh tokens to their session via the current pointer
UPDATE refresh_tokens rt
SET session_id = s.id
FROM user_sessions s
WHERE s.refresh_token_id = rt.id
  AND rt.session_id IS NULL;

-- tokens with no matching session get no session_id (historical orphans — acceptable)

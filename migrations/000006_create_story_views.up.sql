-- Create story_views table to record which user viewed which story/slide
CREATE TABLE story_views (
    id BIGSERIAL PRIMARY KEY,
    viewer_id BIGINT NOT NULL,
    story_id BIGINT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    slide_index INT,
    viewed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX story_views_unique_idx ON story_views(viewer_id, story_id, slide_index);

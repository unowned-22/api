package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/meilisearch/meilisearch-go"
	domainsearch "github.com/unowned-22/api/internal/domain/search"
)

const usersIndexUID = "users"

type MeilisearchUserIndex struct {
	client meilisearch.ServiceManager
}

func NewMeilisearchUserIndex(host, apiKey string) (*MeilisearchUserIndex, error) {
	client := meilisearch.New(host, meilisearch.WithAPIKey(apiKey))

	if _, err := client.Health(); err != nil {
		return nil, fmt.Errorf("meilisearch: failed to connect to %s: %w", host, err)
	}

	idx := client.Index(usersIndexUID)

	searchable := []string{"username", "full_name"}
	if _, err := idx.UpdateSearchableAttributes(&searchable); err != nil {
		return nil, fmt.Errorf("meilisearch: failed to set searchable attributes: %w", err)
	}

	displayed := []string{"id", "username", "full_name", "avatar_url"}
	if _, err := idx.UpdateDisplayedAttributes(&displayed); err != nil {
		return nil, fmt.Errorf("meilisearch: failed to set displayed attributes: %w", err)
	}

	return &MeilisearchUserIndex{client: client}, nil
}

func (m *MeilisearchUserIndex) Index(ctx context.Context, doc domainsearch.UserDocument) error {
	idx := m.client.Index(usersIndexUID)
	_, err := idx.AddDocuments([]domainsearch.UserDocument{doc}, nil)
	if err != nil {
		return fmt.Errorf("meilisearch: index user %d: %w", doc.ID, err)
	}
	return nil
}

func (m *MeilisearchUserIndex) Delete(ctx context.Context, userID int64) error {
	idx := m.client.Index(usersIndexUID)
	_, err := idx.DeleteDocument(strconv.FormatInt(userID, 10), nil)
	if err != nil {
		return fmt.Errorf("meilisearch: delete user %d: %w", userID, err)
	}
	return nil
}

func (m *MeilisearchUserIndex) Search(ctx context.Context, query string, limit int) ([]domainsearch.UserDocument, error) {
	idx := m.client.Index(usersIndexUID)
	res, err := idx.Search(query, &meilisearch.SearchRequest{
		Limit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("meilisearch: search users: %w", err)
	}

	out := make([]domainsearch.UserDocument, 0, len(res.Hits))
	for _, hit := range res.Hits {
		doc := domainsearch.UserDocument{}
		payload, err := json.Marshal(hit)
		if err != nil {
			continue
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(payload, &raw); err != nil {
			continue
		}
		if v, ok := raw["id"]; ok {
			var id int64
			if err := json.Unmarshal(v, &id); err == nil {
				doc.ID = id
			}
		}
		if v, ok := raw["username"]; ok {
			var username string
			if err := json.Unmarshal(v, &username); err == nil {
				doc.Username = username
			}
		}
		if v, ok := raw["full_name"]; ok {
			var fullName string
			if err := json.Unmarshal(v, &fullName); err == nil {
				doc.FullName = fullName
			}
		}
		if v, ok := raw["avatar_url"]; ok {
			var avatarURL string
			if err := json.Unmarshal(v, &avatarURL); err == nil {
				doc.AvatarURL = avatarURL
			}
		}
		out = append(out, doc)
	}
	return out, nil
}

var _ domainsearch.UserIndex = (*MeilisearchUserIndex)(nil)

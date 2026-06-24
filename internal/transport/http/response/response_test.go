package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/unowned-22/api/internal/errs"
)

func TestSendErrorMappings(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantStatus     int
		wantCode       string
		wantMessage    string
		wantNotDefault bool
	}{
		{name: "friendship not found", err: errs.ErrFriendshipNotFound, wantStatus: http.StatusNotFound, wantCode: "FRIENDSHIP_NOT_FOUND", wantMessage: "friendship not found", wantNotDefault: true},
		{name: "friendship already exists", err: errs.ErrFriendshipAlreadyExist, wantStatus: http.StatusConflict, wantCode: "FRIENDSHIP_ALREADY_EXISTS", wantMessage: "friendship already exists", wantNotDefault: true},
		{name: "cannot friend yourself", err: errs.ErrCannotFriendYourself, wantStatus: http.StatusUnprocessableEntity, wantCode: "CANNOT_FRIEND_YOURSELF", wantMessage: "cannot send friendship request to yourself", wantNotDefault: true},
		{name: "not addressee", err: errs.ErrNotAddressee, wantStatus: http.StatusForbidden, wantCode: "NOT_ADDRESSEE", wantMessage: "only addressee can perform this action", wantNotDefault: true},
		{name: "not requester", err: errs.ErrNotRequester, wantStatus: http.StatusForbidden, wantCode: "NOT_REQUESTER", wantMessage: "only requester can perform this action", wantNotDefault: true},
		{name: "not friend", err: errs.ErrNotFriend, wantStatus: http.StatusNotFound, wantCode: "NOT_FRIEND", wantMessage: "users are not friends", wantNotDefault: true},
		{name: "session expired", err: errs.ErrSessionExpired, wantStatus: http.StatusUnauthorized, wantCode: "SESSION_EXPIRED", wantMessage: "session has expired", wantNotDefault: true},
		{name: "session revoked", err: errs.ErrSessionRevoked, wantStatus: http.StatusUnauthorized, wantCode: "SESSION_REVOKED", wantMessage: "session has been revoked", wantNotDefault: true},
		{name: "session not found", err: errs.ErrSessionNotFound, wantStatus: http.StatusNotFound, wantCode: "SESSION_NOT_FOUND", wantMessage: "session not found", wantNotDefault: true},
		{name: "device not found", err: errs.ErrDeviceNotFound, wantStatus: http.StatusNotFound, wantCode: "DEVICE_NOT_FOUND", wantMessage: "device not found", wantNotDefault: true},
		{name: "user deactivated", err: errs.ErrUserDeactivated, wantStatus: http.StatusForbidden, wantCode: "USER_DEACTIVATED", wantMessage: "user account is deactivated", wantNotDefault: true},
		{name: "password reset invalid", err: errs.ErrPasswordResetTokenInvalid, wantStatus: http.StatusBadRequest, wantCode: "INVALID_PASSWORD_RESET_TOKEN", wantMessage: "password reset token is invalid or expired", wantNotDefault: true},
		{name: "password reset used", err: errs.ErrPasswordResetTokenUsed, wantStatus: http.StatusBadRequest, wantCode: "PASSWORD_RESET_TOKEN_USED", wantMessage: "password reset token has already been used", wantNotDefault: true},
		{name: "role not found", err: errs.ErrRoleNotFound, wantStatus: http.StatusNotFound, wantCode: "ROLE_NOT_FOUND", wantMessage: "role not found", wantNotDefault: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			SendError(rr, req, tc.err)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status mismatch: got %d want %d", rr.Code, tc.wantStatus)
			}

			var got ErrorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if got.Error.Code != tc.wantCode {
				t.Fatalf("code mismatch: got %q want %q", got.Error.Code, tc.wantCode)
			}
			if got.Error.Message != tc.wantMessage {
				t.Fatalf("message mismatch: got %q want %q", got.Error.Message, tc.wantMessage)
			}
			if tc.wantNotDefault && rr.Code == http.StatusInternalServerError {
				t.Fatal("mapped business error must not return 500")
			}
		})
	}
}

func TestSendErrorUnknownFallsBackTo500(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	SendError(rr, req, errors.New("boom"))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status mismatch: got %d want %d", rr.Code, http.StatusInternalServerError)
	}

	var got ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got.Error.Code != "INTERNAL_SERVER_ERROR" {
		t.Fatalf("code mismatch: got %q", got.Error.Code)
	}
}

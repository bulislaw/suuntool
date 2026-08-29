package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bulislaw/suuntool/internal/api"
)

// decodeJSON is a generic helper that unmarshals JSON and maps errors to *api.Error.
func decodeJSON[T any](b []byte) (*T, error) {
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, &api.Error{Code: "BAD_ENVELOPE", Message: err.Error(), Exit: 5}
	}
	return &v, nil
}

// RemoteUser holds basic profile info returned by the user endpoint.
type RemoteUser struct {
	Username      string `json:"username"`
	UserKey       string `json:"userKey"`
	Email         string `json:"email,omitempty"`
	Country       string `json:"country,omitempty"`
	UUID          string `json:"uuid,omitempty"`
	EmailVerified bool   `json:"emailVerified"`
}

// Pretty returns a multi-line human-readable representation, skipping empty optional fields.
func (u RemoteUser) Pretty() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "username:      %s\n", u.Username)
	fmt.Fprintf(&sb, "userKey:       %s\n", u.UserKey)
	if u.Email != "" {
		fmt.Fprintf(&sb, "email:         %s\n", u.Email)
	}
	if u.Country != "" {
		fmt.Fprintf(&sb, "country:       %s\n", u.Country)
	}
	if u.UUID != "" {
		fmt.Fprintf(&sb, "uuid:          %s\n", u.UUID)
	}
	fmt.Fprintf(&sb, "emailVerified: %v", u.EmailVerified)
	return sb.String()
}

// FollowCounts holds social follow/block counts for a user.
type FollowCounts struct {
	Followers int `json:"followers"`
	Followings int `json:"followings"`
	Blocked    int `json:"blocked"`
	BlockedBy  int `json:"blockedBy"`
}

// Pretty returns a multi-line human-readable representation.
func (f FollowCounts) Pretty() string {
	return fmt.Sprintf(
		"followers:  %d\nfollowings: %d\nblocked:    %d\nblockedBy:  %d",
		f.Followers, f.Followings, f.Blocked, f.BlockedBy,
	)
}

// Whoami returns the profile of the authenticated user.
func Whoami(ctx context.Context, client *api.Client) (*RemoteUser, error) {
	body, err := client.Do(ctx, "GET", "user", nil, nil)
	if err != nil {
		return nil, err
	}
	v, err := api.DecodeAsko[RemoteUser](body)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// Settings returns raw user settings JSON.
func Settings(ctx context.Context, client *api.Client) (json.RawMessage, error) {
	body, err := client.Do(ctx, "GET", "user/settings", nil, nil)
	if err != nil {
		return nil, err
	}
	v, err := api.DecodeAsko[json.RawMessage](body)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// Follow returns the social follow/block counts for the authenticated user.
func Follow(ctx context.Context, client *api.Client) (*FollowCounts, error) {
	body, err := client.Do(ctx, "GET", "user/follow", nil, nil)
	if err != nil {
		return nil, err
	}
	v, err := api.DecodeAsko[FollowCounts](body)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// UserByName looks up a user profile by username.
func UserByName(ctx context.Context, client *api.Client, username string) (*RemoteUser, error) {
	body, err := client.Do(ctx, "GET", "user/name/"+username, nil, nil)
	if err != nil {
		return nil, err
	}
	v, err := api.DecodeAsko[RemoteUser](body)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

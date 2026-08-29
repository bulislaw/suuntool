package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/bulislaw/suuntool/internal/api"
	"github.com/bulislaw/suuntool/internal/auth"
)

// RemoteUserSession is the response returned by /login2 (not ASKO-wrapped).
type RemoteUserSession struct {
	SessionKey     string `json:"sessionkey"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	UserKey        string `json:"userKey"`
	Country        string `json:"country"`
	EmailVerified  bool   `json:"emailVerified"`
}

// Pretty returns a multi-line human-readable representation with a masked sessionkey.
func (s RemoteUserSession) Pretty() string {
	masked := s.SessionKey
	if len(masked) > 6 {
		masked = masked[:6] + "…"
	}
	return fmt.Sprintf(
		"sessionkey:    %s\nusername:      %s\nemail:         %s\nuserKey:       %s\ncountry:       %s\nemailVerified: %v",
		masked, s.Username, s.Email, s.UserKey, s.Country, s.EmailVerified,
	)
}

// Login authenticates against /login2 and returns a RemoteUserSession.
// The caller controls timeout via the context.
func Login(ctx context.Context, baseURL, email, password string) (*RemoteUserSession, error) {
	totp := auth.GenerateTOTP(email, 0)

	params := []auth.Param{
		{Key: "l", Value: email},
		{Key: "p", Value: password},
		{Key: "totp", Value: totp},
	}
	sig := auth.SignParams("login2", params)
	salt := auth.RandomSalt()
	ts := auth.NowMS()

	form := url.Values{}
	form.Set("l", email)
	form.Set("p", password)
	form.Set("totp", totp)
	form.Set("timestamp", strconv.FormatInt(ts, 10))
	form.Set("salt", salt)
	form.Set("signature", sig)

	headers := map[string]string{
		"Content-Type":                      "application/x-www-form-urlencoded;charset=UTF-8",
		"x-login-email-verification-enabled": "true",
	}

	client := api.NewClient(baseURL, "", 0)
	body, err := client.Do(ctx, "POST", "login2", strings.NewReader(form.Encode()), headers)
	if err != nil {
		return nil, err
	}

	var sess RemoteUserSession
	if err := json.Unmarshal(body, &sess); err != nil {
		return nil, &api.Error{Code: "BAD_ENVELOPE", Message: err.Error(), Exit: 5}
	}
	return &sess, nil
}

// Logout invalidates the session server-side.
func Logout(ctx context.Context, baseURL, sessionKey string) error {
	client := api.NewClient(baseURL, sessionKey, 0)
	_, err := client.Do(ctx, "GET", "logout", nil, nil)
	return err
}

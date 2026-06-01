package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type TokenStore struct {
	Token           string `json:"token"`
	Email           string `json:"email"`
	ExpiresAt       int64  `json:"expires_at"`
	RefreshToken    string `json:"refresh_token,omitempty"`
	FirebaseAPIKey  string `json:"firebase_api_key,omitempty"`
	AppURL          string `json:"app_url,omitempty"`
	APIBaseURL      string `json:"api_base_url,omitempty"`
	ActiveOrgID     string `json:"active_organization_id,omitempty"`
	ActiveProjectID string `json:"active_project_id,omitempty"`
}

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".aila")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func SaveToken(t *TokenStore) error {
	if t == nil {
		return errors.New("nil token")
	}
	p, err := defaultConfigPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(t)
}

func ClearToken() error {
	p, err := defaultConfigPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func LoadToken() (*TokenStore, error) {
	p, err := defaultConfigPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var t TokenStore
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	if t.Token == "" {
		return nil, errors.New("token not found")
	}
	if t.ExpiresAt > 0 && time.Now().Add(60*time.Second).Unix() >= t.ExpiresAt {
		if err := refreshFirebaseToken(&t); err == nil {
			_ = SaveToken(&t)
			return &t, nil
		}
		return nil, errors.New("token expired, please login again")
	}
	return &t, nil
}

func refreshFirebaseToken(t *TokenStore) error {
	if t.RefreshToken == "" || t.FirebaseAPIKey == "" {
		return errors.New("refresh token missing")
	}
	body := []byte(fmt.Sprintf(
		`{"grant_type":"refresh_token","refresh_token":%q}`,
		t.RefreshToken,
	))
	req, err := http.NewRequest(
		http.MethodPost,
		"https://securetoken.googleapis.com/v1/token?key="+t.FirebaseAPIKey,
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("firebase refresh HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var payload struct {
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    string `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return err
	}
	if payload.IDToken == "" {
		return errors.New("firebase refresh did not return id_token")
	}
	expiresIn, _ := strconv.Atoi(payload.ExpiresIn)
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	t.Token = payload.IDToken
	if payload.RefreshToken != "" {
		t.RefreshToken = payload.RefreshToken
	}
	t.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).Unix()
	return nil
}

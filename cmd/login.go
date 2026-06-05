package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ai-la/cli/internal/auth"
	"github.com/ai-la/cli/internal/output"
	"github.com/spf13/cobra"
)

var loginAppURL string
var loginAPIURL string
var loginDev bool

const prodAPIURL = "https://llm-eval-api-530874889975.europe-west1.run.app"

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to your Heimdal account",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogin()
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginAppURL, "app-url", "", "Web app URL for login")
	loginCmd.Flags().StringVar(&loginAPIURL, "api-url", "", "CLI API base URL")
	loginCmd.Flags().BoolVar(&loginDev, "dev", false, "Use dev URLs (app: http://localhost, api: http://localhost:5002)")
}

// Backend'in POST ettiği body.
type callbackPayload struct {
	Token          string `json:"token"`
	Email          string `json:"email"`
	State          string `json:"state"`
	ExpiresAt      int64  `json:"expires_at"`
	RefreshToken   string `json:"refresh_token"`
	FirebaseAPIKey string `json:"firebase_api_key"`
}

func runLogin() error {
	// Create step tracker for styled progress
	tracker := output.NewStepTracker()
	tracker.Add("Starting localhost callback server")
	tracker.Add("Generating CSRF state token")
	tracker.Add("Opening browser")
	tracker.Add("Login URL")
	tracker.Add("Waiting for sign-in")
	tracker.Add("Saving token")

	fmt.Println()
	tracker.Start(0)

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		tracker.Error(0, fmt.Sprintf("Error: %v", err))
		fmt.Println(tracker.Render())
		return fmt.Errorf("failed to start local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	tracker.Done(0, fmt.Sprintf("localhost:%d", port))

	tracker.Start(1)
	stateBytes := make([]byte, 12)
	if _, err := rand.Read(stateBytes); err != nil {
		tracker.Error(1, fmt.Sprintf("Error: %v", err))
		fmt.Println(tracker.Render())
		return fmt.Errorf("failed to generate state token: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	tracker.Done(1, "✓")

	tokenCh := make(chan auth.TokenStore, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{Addr: fmt.Sprintf("localhost:%d", port), Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload callbackPayload
		contentType := strings.ToLower(r.Header.Get("Content-Type"))
		if strings.Contains(contentType, "application/x-www-form-urlencoded") || strings.Contains(contentType, "multipart/form-data") {
			if err := r.ParseForm(); err != nil {
				errCh <- fmt.Errorf("invalid form payload: %w", err)
				return
			}
			payload = callbackPayload{
				Token:          r.FormValue("token"),
				Email:          r.FormValue("email"),
				State:          r.FormValue("state"),
				RefreshToken:   r.FormValue("refresh_token"),
				FirebaseAPIKey: r.FormValue("firebase_api_key"),
			}
			if expiresAt := strings.TrimSpace(r.FormValue("expires_at")); expiresAt != "" {
				if parsed, err := strconv.ParseInt(expiresAt, 10, 64); err == nil {
					payload.ExpiresAt = parsed
				}
			}
		} else {
			body, err := io.ReadAll(io.LimitReader(r.Body, 16384))
			if err != nil {
				errCh <- fmt.Errorf("failed to read request body: %w", err)
				return
			}
			defer r.Body.Close()
			if err := json.Unmarshal(body, &payload); err != nil {
				errCh <- fmt.Errorf("invalid JSON: %w", err)
				return
			}
		}

		if payload.State != state {
			http.Error(w, "invalid state", http.StatusUnauthorized)
			errCh <- fmt.Errorf("invalid state, possible CSRF attempt")
			return
		}

		if payload.Token == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			errCh <- fmt.Errorf("token missing in callback payload")
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!doctype html><title>Heimdal CLI Login</title><body style=\"font-family: system-ui; padding: 32px;\"><h2>CLI login complete.</h2><p>You can return to your terminal.</p></body>"))
		expiresAt := payload.ExpiresAt
		if expiresAt <= 0 {
			expiresAt = time.Now().Add(55 * time.Minute).Unix()
		}
		tokenCh <- auth.TokenStore{
			Token:          payload.Token,
			Email:          payload.Email,
			ExpiresAt:      expiresAt,
			RefreshToken:   payload.RefreshToken,
			FirebaseAPIKey: payload.FirebaseAPIKey,
			AppURL:         loginAppURL,
			APIBaseURL:     loginAPIURL,
		}
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	loginAppURL, loginAPIURL = resolveLoginURLs()
	authURL := fmt.Sprintf("%s/auth/cli?p=%d&s=%s", strings.TrimRight(loginAppURL, "/"), port, state)

	tracker.Start(2)
	if err := openBrowser(authURL); err != nil {
		tracker.Warn(2, "Open manually")
	} else {
		tracker.Done(2, "Opened")
	}
	tracker.Done(3, "Printed below")
	fmt.Println()
	fmt.Println("Login URL:")
	fmt.Println(authURL)
	fmt.Println()

	tracker.Start(4)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case ts := <-tokenCh:
			server.Shutdown(context.Background())
			tracker.Done(4, "Token received")
			tracker.Start(5)

			// Keep existing org/project context across re-login when possible.
			if prev, err := auth.LoadToken(); err == nil && prev != nil {
				ts.ActiveOrgID = prev.ActiveOrgID
				ts.ActiveProjectID = prev.ActiveProjectID
			}

			if err := auth.SaveToken(&ts); err != nil {
				tracker.Error(5, fmt.Sprintf("Error: %v", err))
				fmt.Println(tracker.Render())
				return fmt.Errorf("failed to save token: %w", err)
			}
			tracker.Done(5, ts.Email)
			fmt.Println()
			fmt.Println(tracker.Render())
			fmt.Println()
			return nil

		case err := <-errCh:
			server.Shutdown(context.Background())
			tracker.Error(4, fmt.Sprintf("Error: %v", err))
			fmt.Println(tracker.Render())
			return err

		case <-ticker.C:
			// Silent tick

		case <-ctx.Done():
			server.Shutdown(context.Background())
			tracker.Error(4, "Timeout: sign-in was not completed within 5 minutes")
			fmt.Println(tracker.Render())
			return fmt.Errorf("timeout: sign-in was not completed within 5 minutes")
		}
	}
}

func resolveLoginURLs() (string, string) {
	if loginDev || strings.EqualFold(os.Getenv("HEIMDAL_ENV"), "dev") || os.Getenv("HEIMDAL_DEV") == "1" {
		appURL := firstNonEmptyString(loginAppURL, os.Getenv("HEIMDAL_APP_URL"), "http://localhost")
		apiURL := firstNonEmptyString(loginAPIURL, os.Getenv("HEIMDAL_API_URL"), "http://localhost:5002")
		return appURL, apiURL
	}
	appURL := firstNonEmptyString(loginAppURL, os.Getenv("HEIMDAL_APP_URL"), "https://ailab.co-one.co")
	apiURL := firstNonEmptyString(loginAPIURL, os.Getenv("HEIMDAL_API_URL"), prodAPIURL)
	return appURL, apiURL
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

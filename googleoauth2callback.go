package googleoauth2callback

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
)

type Credentials struct {
	Web struct {
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		AuthURI      string   `json:"auth_uri"`
		TokenURI     string   `json:"token_uri"`
		RedirectURIs []string `json:"redirect_uris"`
	} `json:"web"`
}

type OAuth2Callback struct {
	redirectURL     string
	tokenPath       string
	credentialsPath string
	scopes          []string
}

type Option func(*OAuth2Callback)

func WithRedirectURL(url string) Option {
	return func(o *OAuth2Callback) {
		o.redirectURL = url
	}
}

func WithTokenPath(path string) Option {
	return func(o *OAuth2Callback) {
		o.tokenPath = path
	}
}

func WithCredentialsPath(path string) Option {
	return func(o *OAuth2Callback) {
		o.credentialsPath = path
	}
}

func WithScopes(scopes []string) Option {
	return func(o *OAuth2Callback) {
		o.scopes = scopes
	}
}

func New(opts ...Option) *OAuth2Callback {
	callback := &OAuth2Callback{
		redirectURL:     "http://localhost:4567/callback",
		tokenPath:       "./token.json",
		credentialsPath: "./credentials.json",
		scopes:          []string{},
	}

	for _, opt := range opts {
		opt(callback)
	}

	return callback
}

func (o *OAuth2Callback) parseRedirectURL() (string, string, error) {
	u, err := url.Parse(o.redirectURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse redirect URL: %v", err)
	}

	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	return port, u.Path, nil
}

func (o *OAuth2Callback) GetClient() (*http.Client, error) {
	config, err := o.createOAuth2Config()
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth2 config: %v", err)
	}

	tok, err := o.tokenFromFile()
	if err != nil {
		if err := o.authenticate(); err != nil {
			return nil, fmt.Errorf("authenticate failed: %v", err)
		}
		tok, err = o.tokenFromFile()
		if err != nil {
			return nil, fmt.Errorf("failed to read token file: %v", err)
		}
	}
	return config.Client(context.Background(), tok), nil
}

func (o *OAuth2Callback) tokenFromFile() (*oauth2.Token, error) {
	b, err := os.ReadFile(o.tokenPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read token file: %v", err)
	}
	var tok oauth2.Token
	if err := json.Unmarshal(b, &tok); err != nil {
		return nil, fmt.Errorf("unable to parse token file: %v", err)
	}
	return &tok, nil
}

func (o *OAuth2Callback) createOAuth2Config() (*oauth2.Config, error) {
	absPath, err := filepath.Abs(o.credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %v", err)
	}
	b, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}
	var creds Credentials
	if err := json.Unmarshal(b, &creds); err != nil {
		return nil, fmt.Errorf("unable to parse client secret file: %v", err)
	}
	config := &oauth2.Config{
		ClientID:     creds.Web.ClientID,
		ClientSecret: creds.Web.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  creds.Web.AuthURI,
			TokenURL: creds.Web.TokenURI,
		},
		RedirectURL: o.redirectURL,
		Scopes:      o.scopes,
	}
	return config, nil
}

func generateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (o *OAuth2Callback) authenticate() error {
	port, callbackPath, err := o.parseRedirectURL()
	if err != nil {
		return err
	}

	config, err := o.createOAuth2Config()
	if err != nil {
		return err
	}

	done := make(chan error)

	stateToken, err := generateStateToken()
	if err != nil {
		return fmt.Errorf("failed to generate state token: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		if state != stateToken {
			http.Error(w, "Invalid state token", http.StatusBadRequest)
			done <- fmt.Errorf("invalid state token")
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Code not found", http.StatusBadRequest)
			done <- fmt.Errorf("code not found in request")
			return
		}
		ctx := context.Background()
		token, err := config.Exchange(ctx, code)
		if err != nil {
			http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
			done <- fmt.Errorf("failed to exchange token: %v", err)
			return
		}
		tokenJSON, err := json.Marshal(token)
		if err != nil {
			http.Error(w, "Failed to serialize token", http.StatusInternalServerError)
			done <- fmt.Errorf("failed to marshal token: %v", err)
			return
		}
		absTokenPath, err := filepath.Abs(o.tokenPath)
		if err != nil {
			http.Error(w, "Failed to get token path", http.StatusInternalServerError)
			done <- fmt.Errorf("failed to get absolute token path: %v", err)
			return
		}
		if err := os.WriteFile(absTokenPath, tokenJSON, 0644); err != nil {
			http.Error(w, "Failed to write token file", http.StatusInternalServerError)
			done <- fmt.Errorf("failed to write token file: %v", err)
			return
		}
		fmt.Fprintf(w, "Authentication successful! You can close this tab and return to the console.")
		done <- nil
	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	serverError := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stderr, "Starting server on port %s\n", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverError <- fmt.Errorf("ListenAndServe error: %v", err)
		}
		close(serverError)
	}()

	authURL := config.AuthCodeURL(stateToken,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce)
	fmt.Fprintln(os.Stderr, "Authenticate this app by visiting this url:")
	fmt.Fprintln(os.Stderr, authURL)

	err = <-done

	if err := srv.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Server close error: %v\n", err)
	}

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if errShutdown := srv.Shutdown(ctxShutdown); errShutdown != nil && errShutdown != context.DeadlineExceeded {
		fmt.Fprintf(os.Stderr, "Server shutdown error: %v\n", errShutdown)
	}

	if serverErr := <-serverError; serverErr != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", serverErr)
	}

	return err
}

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const playlistSlots = 4

type playlist struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type googleDesktopClient struct {
	ID     string `json:"client_id"`
	Secret string `json:"client_secret"`
}

func importPlaylists(credentialsPath, configPath string) error {
	credentials, err := os.ReadFile(credentialsPath)
	if err != nil {
		return fmt.Errorf("read Google OAuth client JSON: %w", err)
	}
	var clientFile struct {
		Installed googleDesktopClient `json:"installed"`
	}
	if err := json.Unmarshal(credentials, &clientFile); err != nil || clientFile.Installed.ID == "" || clientFile.Installed.Secret == "" {
		return fmt.Errorf("use the JSON downloaded for a Google OAuth Desktop app client")
	}
	original, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read installed playlists.json: %w", err)
	}
	var settings config
	if err := json.Unmarshal(original, &settings); err != nil {
		return fmt.Errorf("parse installed playlists.json: %w", err)
	}
	token, err := authorizePlaylistImport(clientFile.Installed)
	if err != nil {
		return err
	}
	owned, err := ownedPlaylists(token)
	if err != nil {
		return err
	}
	if len(owned) == 0 {
		fmt.Println("No owned playlists found.")
		return nil
	}
	for i, p := range owned {
		fmt.Printf("%d. %s\n", i+1, p.Name)
	}
	input := bufio.NewScanner(os.Stdin)
	changed := false
	for i := 0; i < playlistSlots; i++ {
		current := "(empty)"
		if i < len(settings.Playlists) {
			current = settings.Playlists[i].Name
		}
		for {
			fmt.Printf("Menu slot %d (%s): playlist number, Enter to keep, or q to cancel: ", i+1, current)
			if !input.Scan() {
				return input.Err()
			}
			choice := strings.TrimSpace(input.Text())
			if strings.EqualFold(choice, "q") {
				return nil
			}
			if choice == "" {
				break
			}
			number, err := strconv.Atoi(choice)
			if err != nil || number < 1 || number > len(owned) {
				fmt.Println("Enter a number from the list.")
				continue
			}
			for len(settings.Playlists) <= i {
				settings.Playlists = append(settings.Playlists, playlist{})
			}
			settings.Playlists[i] = owned[number-1]
			changed = true
			break
		}
	}
	if !changed {
		return nil
	}
	current, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, original) {
		return fmt.Errorf("playlists.json changed during import; run the import again")
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := saveBytesAtomic(configPath, append(data, '\n')); err != nil {
		return err
	}
	fmt.Println("Saved playlist selections. Reopen the Rainmeter menu to see them.")
	return nil
}

func authorizePlaylistImport(google googleDesktopClient) (string, error) {
	random := make([]byte, 64)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(random[:32])
	verifier := base64.RawURLEncoding.EncodeToString(random[32:])
	challenge := sha256.Sum256([]byte(verifier))
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	redirect := "http://" + listener.Addr().String() + "/oauth2callback"
	type authorizationResult struct {
		code string
		err  error
	}
	result := make(chan authorizationResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			return
		}
		outcome := authorizationResult{code: q.Get("code")}
		if outcome.code == "" || q.Get("error") != "" {
			outcome.err = fmt.Errorf("Google authorization did not complete")
		}
		fmt.Fprintln(w, "Return to the Ambient Music terminal to continue.")
		select {
		case result <- outcome:
		default:
		}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go server.Serve(listener)
	authQuery := url.Values{
		"client_id": {google.ID}, "redirect_uri": {redirect}, "response_type": {"code"},
		"scope": {"https://www.googleapis.com/auth/youtube.readonly"}, "state": {state},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"},
	}
	fmt.Println("Open this URL in your browser and approve YouTube read access:")
	fmt.Println("https://accounts.google.com/o/oauth2/v2/auth?" + authQuery.Encode())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("Google authorization timed out")
	case outcome := <-result:
		if outcome.err != nil {
			return "", outcome.err
		}
		response, err := client.PostForm("https://oauth2.googleapis.com/token", url.Values{
			"client_id": {google.ID}, "client_secret": {google.Secret}, "redirect_uri": {redirect},
			"grant_type": {"authorization_code"}, "code": {outcome.code}, "code_verifier": {verifier},
		})
		if err != nil {
			return "", fmt.Errorf("exchange Google authorization code: %w", err)
		}
		defer response.Body.Close()
		var token struct {
			AccessToken string `json:"access_token"`
		}
		if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&token) != nil || token.AccessToken == "" {
			return "", fmt.Errorf("Google token exchange failed (HTTP %d)", response.StatusCode)
		}
		return token.AccessToken, nil
	}
}

func ownedPlaylists(token string) ([]playlist, error) {
	query := url.Values{"part": {"snippet"}, "mine": {"true"}, "maxResults": {"50"}}
	var owned []playlist
	for {
		req, err := http.NewRequest("GET", "https://www.googleapis.com/youtube/v3/playlists?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		var page struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title string `json:"title"`
				} `json:"snippet"`
			} `json:"items"`
			Next string `json:"nextPageToken"`
		}
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("YouTube playlist listing failed (HTTP %d); check that YouTube Data API v3 is enabled", resp.StatusCode)
		}
		if err != nil {
			return nil, fmt.Errorf("decode YouTube playlists: %w", err)
		}
		for _, item := range page.Items {
			if item.ID != "" {
				owned = append(owned, playlist{Name: strings.Join(strings.Fields(item.Snippet.Title), " "), ID: item.ID})
			}
		}
		if page.Next == "" {
			return owned, nil
		}
		query.Set("pageToken", page.Next)
	}
}

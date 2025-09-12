package services

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type TsResp struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}

func VerifyTurnstile(ctx context.Context, token, remoteIP string) (TsResp, error) {
	form := url.Values{}
	form.Set("secret", os.Getenv("CF_TURNSTILE_SECRET"))
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	cli := &http.Client{Timeout: 4 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return TsResp{}, err
	}
	defer resp.Body.Close()
	log.Println(resp)
	log.Println(os.Getenv("CF_TURNSTILE_SECRET"))

	var out TsResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return TsResp{}, err
	}
	return out, nil
}

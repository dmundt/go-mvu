package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
)

type config struct {
	BaseURL      string
	Issuer       string
	ClientID     string
	ClientSecret string
	DataDir      string
	SessionKey   []byte
	CSRFKey      []byte
	Development  bool
}

func loadConfig() (config, error) {
	cfg := config{
		BaseURL:      os.Getenv("GOMVU_BASE_URL"),
		Issuer:       os.Getenv("OIDC_ISSUER"),
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		DataDir:      os.Getenv("GOMVU_DATA_DIR"),
		Development:  os.Getenv("GOMVU_DEVELOPMENT") == "true",
	}
	var err error
	cfg.SessionKey, err = decodeKey("GOMVU_SESSION_KEY")
	if err != nil {
		return config{}, err
	}
	cfg.CSRFKey, err = decodeKey("GOMVU_CSRF_KEY")
	if err != nil {
		return config{}, err
	}
	return cfg, nil
}

func decodeKey(name string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(os.Getenv(name))
	if err != nil || len(key) < 32 {
		return nil, fmt.Errorf("%s must contain base64-encoded random bytes (at least 32)", name)
	}
	return key, nil
}

func validateOrigin(raw string, development bool, issuer bool) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" ||
		u.Fragment != "" || (!issuer && u.Path != "" && u.Path != "/") {
		return nil, errors.New("invalid origin or issuer URL")
	}
	local := u.Hostname() == "localhost"
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil {
		local = ip.IsLoopback()
	}
	if u.Scheme != "https" && !(development && local && u.Scheme == "http") {
		return nil, errors.New("HTTPS required (HTTP allowed only for explicit loopback development)")
	}
	return u, nil
}

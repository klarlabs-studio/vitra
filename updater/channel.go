package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// DefaultMaxBytes caps channel downloads (manifest + artifact).
const DefaultMaxBytes = 64 << 20 // 64 MiB

// ChannelSource describes a conventional HTTP(S) update channel layout:
//
//	{BaseURL}/{AppID}/{Channel}/manifest.json
//	{BaseURL}/{AppID}/{Channel}/{Artifact}
//
// This is a fetch client only — Vitra does not host a CDN.
type ChannelSource struct {
	BaseURL string
	AppID   string
	Channel Channel
}

// HTTPDoer is satisfied by *http.Client (and test fakes).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Fetcher downloads signed update manifests (and optional artifacts) from a
// ChannelSource. Verification remains the caller's responsibility via
// VerifyManifest / PlanInstall (invariant 9).
type Fetcher struct {
	Client   HTTPDoer
	MaxBytes int64
}

func (f *Fetcher) client() HTTPDoer {
	if f != nil && f.Client != nil {
		return f.Client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (f *Fetcher) maxBytes() int64 {
	if f != nil && f.MaxBytes > 0 {
		return f.MaxBytes
	}
	return DefaultMaxBytes
}

// ManifestURL returns the conventional manifest URL for the channel.
func (s ChannelSource) ManifestURL() (string, error) {
	base, err := s.validatedBase()
	if err != nil {
		return "", err
	}
	appID, channel, err := s.validatedIdentity()
	if err != nil {
		return "", err
	}
	u := *base
	u.Path = path.Join(strings.TrimSuffix(base.Path, "/"), appID, channel, "manifest.json")
	return u.String(), nil
}

// ArtifactURL resolves an artifact name under the channel prefix.
// Absolute http(s) Artifact values are accepted as-is after scheme checks.
func (s ChannelSource) ArtifactURL(artifact string) (string, error) {
	artifact = strings.TrimSpace(artifact)
	if artifact == "" {
		return "", errors.New("artifact name is required")
	}
	if strings.Contains(artifact, "://") {
		u, err := url.Parse(artifact)
		if err != nil {
			return "", fmt.Errorf("artifact url: %w", err)
		}
		if err := validateHTTPScheme(u); err != nil {
			return "", err
		}
		return u.String(), nil
	}
	if err := validateRelativeArtifact(artifact); err != nil {
		return "", err
	}
	base, err := s.validatedBase()
	if err != nil {
		return "", err
	}
	appID, channel, err := s.validatedIdentity()
	if err != nil {
		return "", err
	}
	u := *base
	u.Path = path.Join(strings.TrimSuffix(base.Path, "/"), appID, channel, path.Clean(artifact))
	return u.String(), nil
}

func (s ChannelSource) validatedBase() (*url.URL, error) {
	raw := strings.TrimSpace(s.BaseURL)
	if raw == "" {
		return nil, errors.New("channel base URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("channel base URL: %w", err)
	}
	if err := validateHTTPScheme(u); err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, errors.New("channel base URL must include a host")
	}
	return u, nil
}

func (s ChannelSource) validatedIdentity() (appID, channel string, err error) {
	appID = strings.TrimSpace(s.AppID)
	if appID == "" {
		return "", "", errors.New("channel app id is required")
	}
	if strings.ContainsAny(appID, "/\\") || strings.Contains(appID, "..") {
		return "", "", errors.New("channel app id must not contain path separators")
	}
	channel = strings.TrimSpace(string(s.Channel))
	if channel == "" {
		return "", "", errors.New("channel name is required")
	}
	if strings.ContainsAny(channel, "/\\") || strings.Contains(channel, "..") {
		return "", "", errors.New("channel name must not contain path separators")
	}
	return appID, channel, nil
}

func validateHTTPScheme(u *url.URL) error {
	switch strings.ToLower(u.Scheme) {
	case "https", "http":
		return nil
	default:
		return fmt.Errorf("channel URL scheme %q rejected (want http or https)", u.Scheme)
	}
}

func validateRelativeArtifact(artifact string) error {
	if path.IsAbs(artifact) || strings.HasPrefix(artifact, `\`) {
		return errors.New("relative artifact must not be absolute")
	}
	clean := path.Clean(artifact)
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return errors.New("artifact path must not escape the channel prefix")
	}
	if strings.Contains(artifact, `\`) {
		return errors.New("artifact path must use forward slashes")
	}
	return nil
}

// FetchManifest downloads and decodes the channel manifest.json (unsigned decode only).
func (f *Fetcher) FetchManifest(ctx context.Context, src ChannelSource) (Manifest, error) {
	manifestURL, err := src.ManifestURL()
	if err != nil {
		return Manifest{}, err
	}
	body, err := f.get(ctx, manifestURL)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if m.AppID == "" {
		m.AppID = strings.TrimSpace(src.AppID)
	}
	if m.Channel == "" {
		m.Channel = src.Channel
	}
	return m, nil
}

// FetchArtifact downloads bytes for m.Artifact under the channel layout.
func (f *Fetcher) FetchArtifact(ctx context.Context, src ChannelSource, m Manifest) ([]byte, error) {
	artifactURL, err := src.ArtifactURL(m.Artifact)
	if err != nil {
		return nil, err
	}
	return f.get(ctx, artifactURL)
}

func (f *Fetcher) get(ctx context.Context, rawURL string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, application/octet-stream, */*")
	resp, err := f.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: HTTP %s", rawURL, resp.Status)
	}
	limited := io.LimitReader(resp.Body, f.maxBytes()+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > f.maxBytes() {
		return nil, fmt.Errorf("response exceeds %d byte limit", f.maxBytes())
	}
	return body, nil
}

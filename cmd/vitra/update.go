package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/policy"
	"go.klarlabs.de/vitra/updater"
)

func runUpdateApply(args []string) error {
	manifestPath, artifactPath, pubkeyHex, dest, policyEnv := "", "", "", "", ""
	baseURL, appID, channel, currentVersion := "", "", "stable", ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--manifest":
			i++
			if i >= len(args) {
				return fmt.Errorf("--manifest requires a path")
			}
			manifestPath = args[i]
		case "--artifact":
			i++
			if i >= len(args) {
				return fmt.Errorf("--artifact requires a path")
			}
			artifactPath = args[i]
		case "--base-url":
			i++
			if i >= len(args) {
				return fmt.Errorf("--base-url requires a URL")
			}
			baseURL = args[i]
		case "--app-id":
			i++
			if i >= len(args) {
				return fmt.Errorf("--app-id requires an id")
			}
			appID = args[i]
		case "--channel":
			i++
			if i >= len(args) {
				return fmt.Errorf("--channel requires a name")
			}
			channel = args[i]
		case "--pubkey":
			i++
			if i >= len(args) {
				return fmt.Errorf("--pubkey requires hex-encoded ed25519 public key")
			}
			pubkeyHex = args[i]
		case "--dest":
			i++
			if i >= len(args) {
				return fmt.Errorf("--dest requires a path")
			}
			dest = args[i]
		case "--policy":
			i++
			if i >= len(args) {
				return fmt.Errorf("--policy requires production or development")
			}
			policyEnv = args[i]
		case "--current-version":
			i++
			if i >= len(args) {
				return fmt.Errorf("--current-version requires the installed version")
			}
			currentVersion = args[i]
		default:
			return fmt.Errorf("unknown update-apply flag %q", args[i])
		}
	}
	usage := "usage: vitra update-apply (--manifest <json> --artifact <path> | --base-url <url>) --app-id <id> [--channel name] --current-version <semver> --pubkey <hex> --dest <path> [--policy production|development]"
	if pubkeyHex == "" || dest == "" || appID == "" || currentVersion == "" {
		return fmt.Errorf("%s", usage)
	}
	localMode := manifestPath != "" || artifactPath != ""
	channelMode := baseURL != ""
	if localMode && channelMode {
		return fmt.Errorf("update-apply: use either local --manifest/--artifact or channel --base-url, not both")
	}
	if localMode && (manifestPath == "" || artifactPath == "") {
		return fmt.Errorf("%s", usage)
	}
	if !localMode && !channelMode {
		return fmt.Errorf("%s", usage)
	}

	pubBytes, err := hex.DecodeString(strings.TrimSpace(pubkeyHex))
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("pubkey must be %d-byte ed25519 key as hex", ed25519.PublicKeySize)
	}

	var m updater.Manifest
	var artifact []byte
	if channelMode {
		src := updater.ChannelSource{
			BaseURL: baseURL,
			AppID:   appID,
			Channel: updater.Channel(channel),
		}
		f := &updater.Fetcher{}
		m, err = f.FetchManifest(context.Background(), src)
		if err != nil {
			return err
		}
		artifact, err = f.FetchArtifact(context.Background(), src, m)
		if err != nil {
			return err
		}
	} else {
		rawManifest, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(rawManifest, &m); err != nil {
			return fmt.Errorf("manifest: %w", err)
		}
		artifact, err = os.ReadFile(artifactPath)
		if err != nil {
			return err
		}
	}

	rt, err := vitra.New(vitra.Config{AppID: domain.AppID(appID)})
	if err != nil {
		return fmt.Errorf("--app-id: %w", err)
	}
	if policyEnv != "" {
		var env policy.Environment
		switch policyEnv {
		case string(policy.EnvProduction):
			env = policy.EnvProduction
		case string(policy.EnvDevelopment):
			env = policy.EnvDevelopment
		default:
			return fmt.Errorf("--policy: want %q or %q", policy.EnvProduction, policy.EnvDevelopment)
		}
		eng, err := policy.NewEngine(policy.Document{}, env)
		if err != nil {
			return err
		}
		rt.SetPolicy(eng)
	}
	installed := updater.Installed{AppID: appID, Channel: updater.Channel(channel), Version: currentVersion}
	plan, err := rt.ApplyUpdate(m, ed25519.PublicKey(pubBytes), artifact, dest, installed)
	if err != nil {
		return err
	}
	fmt.Printf("installed %s v%s (%s) → %s\n  sha256: %s\n", plan.AppID, plan.Version, plan.Channel, dest, plan.SHA256)
	return nil
}

func runUpdateCheck(args []string) error {
	baseURL, appID, channel, pubkeyHex := "", "", "stable", ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--base-url":
			i++
			if i >= len(args) {
				return fmt.Errorf("--base-url requires a URL")
			}
			baseURL = args[i]
		case "--app-id":
			i++
			if i >= len(args) {
				return fmt.Errorf("--app-id requires an id")
			}
			appID = args[i]
		case "--channel":
			i++
			if i >= len(args) {
				return fmt.Errorf("--channel requires a name")
			}
			channel = args[i]
		case "--pubkey":
			i++
			if i >= len(args) {
				return fmt.Errorf("--pubkey requires hex-encoded ed25519 public key")
			}
			pubkeyHex = args[i]
		default:
			return fmt.Errorf("unknown update-check flag %q", args[i])
		}
	}
	if baseURL == "" || appID == "" || pubkeyHex == "" {
		return fmt.Errorf("usage: vitra update-check --base-url <url> --app-id <id> --channel <name> --pubkey <hex>")
	}
	pubBytes, err := hex.DecodeString(strings.TrimSpace(pubkeyHex))
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("pubkey must be %d-byte ed25519 key as hex", ed25519.PublicKeySize)
	}
	src := updater.ChannelSource{
		BaseURL: baseURL,
		AppID:   appID,
		Channel: updater.Channel(channel),
	}
	manifestURL, err := src.ManifestURL()
	if err != nil {
		return err
	}
	f := &updater.Fetcher{}
	m, err := f.FetchManifest(context.Background(), src)
	if err != nil {
		return err
	}
	if err := updater.VerifyManifest(m, ed25519.PublicKey(pubBytes)); err != nil {
		return err
	}
	fmt.Printf("update available: %s v%s (%s)\n  manifest: %s\n  artifact: %s\n  sha256: %s\n",
		m.AppID, m.Version, m.Channel, manifestURL, m.Artifact, m.SHA256)
	return nil
}

func runUpdateStage(args []string) error {
	outDir, manifestPath, artifactPath := "", "", ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			i++
			if i >= len(args) {
				return fmt.Errorf("--out requires a directory")
			}
			outDir = args[i]
		case "--manifest":
			i++
			if i >= len(args) {
				return fmt.Errorf("--manifest requires a path")
			}
			manifestPath = args[i]
		case "--artifact":
			i++
			if i >= len(args) {
				return fmt.Errorf("--artifact requires a path")
			}
			artifactPath = args[i]
		default:
			return fmt.Errorf("unknown update-stage flag %q", args[i])
		}
	}
	if outDir == "" || manifestPath == "" || artifactPath == "" {
		return fmt.Errorf("usage: vitra update-stage --out <dir> --manifest <json> --artifact <path>")
	}
	rawManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var m updater.Manifest
	if err := json.Unmarshal(rawManifest, &m); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	stage, err := updater.StageChannel(outDir, m, artifact)
	if err != nil {
		return err
	}
	fmt.Printf("staged update channel\n  root:     %s\n  manifest: %s\n  artifact: %s\n  version:  %s (%s)\n",
		stage.Root, stage.ManifestPath, stage.ArtifactPath, m.Version, m.Channel)
	return nil
}

func runUpdateKeygen(args []string) error {
	outDir := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			i++
			if i >= len(args) {
				return fmt.Errorf("--out requires a directory")
			}
			outDir = args[i]
		default:
			return fmt.Errorf("unknown update-keygen flag %q", args[i])
		}
	}
	kp, err := updater.GenerateKeyPair()
	if err != nil {
		return err
	}
	if outDir == "" {
		fmt.Printf("update signing key pair\n  public:  %s\n  private: %s\n", kp.PublicHex, kp.PrivateHex)
		fmt.Println("store the private key via env:/file:/secret: refs; never pass bare hex to --privkey")
		return nil
	}
	privPath, pubPath, err := updater.WriteKeyPair(outDir, kp)
	if err != nil {
		return err
	}
	fmt.Printf("wrote update signing key pair\n  private: %s\n  public:  %s\n", privPath, pubPath)
	return nil
}

func runUpdateSign(args []string) error {
	artifactPath, appID, version, privRef, outPath, artifactName := "", "", "", "", "", ""
	channel := string(updater.ChannelStable)
	ttl := updater.DefaultManifestTTL
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			i++
			if i >= len(args) {
				return fmt.Errorf("--artifact requires a path")
			}
			artifactPath = args[i]
		case "--app-id":
			i++
			if i >= len(args) {
				return fmt.Errorf("--app-id requires an id")
			}
			appID = args[i]
		case "--version":
			i++
			if i >= len(args) {
				return fmt.Errorf("--version requires a version")
			}
			version = args[i]
		case "--privkey":
			i++
			if i >= len(args) {
				return fmt.Errorf("--privkey requires env:/file:/secret: ref")
			}
			privRef = args[i]
		case "--out":
			i++
			if i >= len(args) {
				return fmt.Errorf("--out requires a path")
			}
			outPath = args[i]
		case "--channel":
			i++
			if i >= len(args) {
				return fmt.Errorf("--channel requires a name")
			}
			channel = args[i]
		case "--artifact-name":
			i++
			if i >= len(args) {
				return fmt.Errorf("--artifact-name requires a name")
			}
			artifactName = args[i]
		case "--expires-in":
			i++
			if i >= len(args) {
				return fmt.Errorf("--expires-in requires a duration (e.g. 30d, 72h)")
			}
			d, err := parseTTL(args[i])
			if err != nil {
				return fmt.Errorf("--expires-in: %w", err)
			}
			ttl = d
		default:
			return fmt.Errorf("unknown update-sign flag %q", args[i])
		}
	}
	if artifactPath == "" || appID == "" || version == "" || privRef == "" || outPath == "" {
		return fmt.Errorf("usage: vitra update-sign --artifact <path> --app-id <id> --version <ver> --privkey <ref> --out <manifest.json> [--channel stable|beta] [--artifact-name name]")
	}
	if artifactName == "" {
		artifactName = filepath.Base(artifactPath)
	}
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	priv, err := updater.LoadPrivateKeyRef(privRef)
	if err != nil {
		return err
	}
	m, err := updater.BuildSignedManifestTTL(appID, version, updater.Channel(channel), artifactName, artifact, priv, ttl)
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, body, 0o644); err != nil {
		return err
	}
	fmt.Printf("signed update manifest\n  out:      %s\n  app:      %s\n  version:  %s (%s)\n  artifact: %s\n  sha256:   %s\n",
		outPath, m.AppID, m.Version, m.Channel, m.Artifact, m.SHA256)
	return nil
}

// parseTTL accepts Go durations ("72h") and whole days ("30d").
func parseTTL(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid day count %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return d, nil
}

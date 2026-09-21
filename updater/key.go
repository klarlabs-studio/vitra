package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// KeyPair holds a freshly generated ed25519 update-signing key pair as hex.
type KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
	PrivateHex string
	PublicHex  string
}

// GenerateKeyPair creates a new ed25519 key pair for signing update manifests.
func GenerateKeyPair() (KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{
		PrivateKey: priv,
		PublicKey:  pub,
		PrivateHex: hex.EncodeToString(priv),
		PublicHex:  hex.EncodeToString(pub),
	}, nil
}

// WriteKeyPair writes priv.key and pub.key (hex, 0600/0644) under outDir.
func WriteKeyPair(outDir string, kp KeyPair) (privPath, pubPath string, err error) {
	if strings.TrimSpace(outDir) == "" {
		return "", "", errors.New("output directory is required")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", "", err
	}
	privPath = filepath.Join(outDir, "priv.key")
	pubPath = filepath.Join(outDir, "pub.key")
	if err := os.WriteFile(privPath, []byte(kp.PrivateHex+"\n"), 0o600); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(pubPath, []byte(kp.PublicHex+"\n"), 0o644); err != nil {
		return "", "", err
	}
	return privPath, pubPath, nil
}

// DigestArtifact returns the lowercase hex SHA-256 of artifact bytes.
func DigestArtifact(artifact []byte) string {
	sum := sha256.Sum256(artifact)
	return hex.EncodeToString(sum[:])
}

// LoadPrivateKeyRef loads an ed25519 private key from a reference (invariant 10):
//
//	env:<NAME>     — hex key in environment variable NAME
//	file:<path>    — hex key in a file
//	secret:<name>  — hex key in VITRA_SECRET_<NAME> (non-alnum → _)
//
// Bare hex on the argv is rejected.
func LoadPrivateKeyRef(ref string) (ed25519.PrivateKey, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, errors.New("private key ref is required")
	}
	var raw string
	switch {
	case strings.HasPrefix(ref, "env:"):
		name := strings.TrimPrefix(ref, "env:")
		if name == "" {
			return nil, errors.New("env: ref requires a variable name")
		}
		val, set := os.LookupEnv(name)
		if !set || val == "" {
			return nil, fmt.Errorf("env %s is unset or empty", name)
		}
		raw = val
	case strings.HasPrefix(ref, "file:"):
		path := strings.TrimPrefix(ref, "file:")
		if path == "" {
			return nil, errors.New("file: ref requires a path")
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		raw = string(body)
	case strings.HasPrefix(ref, "secret:"):
		name := strings.TrimPrefix(ref, "secret:")
		if name == "" {
			return nil, errors.New("secret: ref requires a name")
		}
		envName := secretEnvKey(name)
		val, set := os.LookupEnv(envName)
		if !set || val == "" {
			return nil, fmt.Errorf("secret %q unset: set %s", name, envName)
		}
		raw = val
	default:
		return nil, fmt.Errorf("private key ref %q must start with env:, file:, or secret: (bare hex rejected)", ref)
	}
	return parsePrivateKeyHex(raw)
}

func parsePrivateKeyHex(raw string) (ed25519.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "0x")
	b, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("private key hex: %w", err)
	}
	if len(b) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("private key must be %d bytes (got %d)", ed25519.PrivateKeySize, len(b))
	}
	return ed25519.PrivateKey(b), nil
}

func secretEnvKey(name string) string {
	var b strings.Builder
	b.WriteString("VITRA_SECRET_")
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 'a' + 'A')
		case unicode.IsUpper(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

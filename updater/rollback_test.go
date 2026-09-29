package updater_test

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/updater"
)

func signedRelease(t *testing.T, appID, version string, channel updater.Channel) (updater.Manifest, ed25519.PublicKey, []byte) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("binary " + version)
	m, err := updater.BuildSignedManifest(appID, version, channel, "app.bin", artifact, priv)
	if err != nil {
		t.Fatal(err)
	}
	return m, pub, artifact
}

func TestPlanInstall_RequiresNewerVersionSameAppAndChannel(t *testing.T) {
	installed := updater.Installed{AppID: "com.example.app", Channel: updater.ChannelStable, Version: "1.4.0"}

	m, pub, artifact := signedRelease(t, "com.example.app", "1.5.0", updater.ChannelStable)
	if _, err := updater.PlanInstall(m, pub, artifact, installed); err != nil {
		t.Fatalf("upgrade rejected: %v", err)
	}

	for name, tc := range map[string]struct {
		appID   string
		version string
		channel updater.Channel
		want    error
	}{
		// A validly signed old release replayed by a mirror or MITM.
		"downgrade":            {"com.example.app", "1.3.9", updater.ChannelStable, updater.ErrNotNewer},
		"reinstall same":       {"com.example.app", "1.4.0", updater.ChannelStable, updater.ErrNotNewer},
		"prerelease of same":   {"com.example.app", "1.4.0-rc.1", updater.ChannelStable, updater.ErrNotNewer},
		"other app, same key":  {"com.example.other", "9.0.0", updater.ChannelStable, updater.ErrWrongApp},
		"beta build on stable": {"com.example.app", "2.0.0-beta.1", updater.ChannelBeta, updater.ErrWrongChannel},
	} {
		t.Run(name, func(t *testing.T) {
			m, pub, artifact := signedRelease(t, tc.appID, tc.version, tc.channel)
			if _, err := updater.PlanInstall(m, pub, artifact, installed); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPlanInstall_RejectsUnparseableVersions(t *testing.T) {
	m, pub, artifact := signedRelease(t, "com.example.app", "latest", updater.ChannelStable)
	_, err := updater.PlanInstall(m, pub, artifact, updater.Installed{AppID: "com.example.app", Channel: updater.ChannelStable, Version: "1.0.0"})
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("err = %v", err)
	}
	m, pub, artifact = signedRelease(t, "com.example.app", "1.0.1", updater.ChannelStable)
	if _, err := updater.PlanInstall(m, pub, artifact, updater.Installed{AppID: "com.example.app", Channel: updater.ChannelStable}); err == nil {
		t.Fatal("empty installed version accepted")
	}
}

func TestCompareVersions(t *testing.T) {
	// SemVer 2.0.0 §11 precedence, ascending.
	ordered := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "v1.0.1", "1.1.0", "2.0.0",
	}
	for i := 0; i+1 < len(ordered); i++ {
		c, err := updater.CompareVersions(ordered[i], ordered[i+1])
		if err != nil || c >= 0 {
			t.Fatalf("%s < %s: got %d, %v", ordered[i], ordered[i+1], c, err)
		}
		c, err = updater.CompareVersions(ordered[i+1], ordered[i])
		if err != nil || c <= 0 {
			t.Fatalf("%s > %s: got %d, %v", ordered[i+1], ordered[i], c, err)
		}
	}
	if c, err := updater.CompareVersions("1.0.0+build.5", "1.0.0"); err != nil || c != 0 {
		t.Fatalf("build metadata must not affect precedence: %d %v", c, err)
	}
	for _, bad := range []string{"", "1", "1.0", "1.0.0.0", "01.0.0", "1.0.0-", "1.0.0-01", "a.b.c", "1.0.0-a..b"} {
		if _, err := updater.CompareVersions(bad, "1.0.0"); err == nil {
			t.Fatalf("%q parsed", bad)
		}
	}
}

func TestChannelSource_RequiresHTTPSExceptLoopback(t *testing.T) {
	for base, ok := range map[string]bool{
		"https://updates.example.com":  true,
		"http://127.0.0.1:8080":        true,
		"http://localhost:8080":        true,
		"http://[::1]:8080":            true,
		"http://updates.example.com":   false,
		"http://127.0.0.1.example.com": false,
	} {
		_, err := updater.ChannelSource{BaseURL: base, AppID: "com.example.app", Channel: updater.ChannelStable}.ManifestURL()
		if (err == nil) != ok {
			t.Errorf("%s: err=%v, want ok=%v", base, err, ok)
		}
	}
}

// Manifest versions are attacker-controlled input until the signature is
// checked; parsing must not panic and precedence must be antisymmetric.
func FuzzCompareVersions(f *testing.F) {
	for _, s := range []string{"1.0.0", "v2.3.4-rc.1+b5", "1.0.0-alpha.1", "0.0.0-0", "18446744073709551616.0.0"} {
		f.Add(s, "1.0.0")
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		ab, errA := updater.CompareVersions(a, b)
		ba, errB := updater.CompareVersions(b, a)
		if (errA == nil) != (errB == nil) {
			t.Fatalf("asymmetric errors: %v / %v", errA, errB)
		}
		if errA == nil && ab != -ba {
			t.Fatalf("Compare(%q,%q)=%d but Compare(%q,%q)=%d", a, b, ab, b, a, ba)
		}
	})
}

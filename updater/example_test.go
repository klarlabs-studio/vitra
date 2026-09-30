package updater_test

import (
	"errors"
	"fmt"
	"time"

	"go.klarlabs.de/vitra/updater"
)

// Sign a release, then decide whether an installed app may take it. Only a
// signed, untampered, unexpired, strictly newer release of the same app and
// channel is accepted.
func Example() {
	keys, err := updater.GenerateKeyPair()
	if err != nil {
		panic(err)
	}
	artifact := []byte("notes 1.3.0 binary")
	m, err := updater.BuildSignedManifestTTL("com.example.notes", "1.3.0", updater.ChannelStable,
		"notes", artifact, keys.PrivateKey, 30*24*time.Hour)
	if err != nil {
		panic(err)
	}

	check := func(label string, artifact []byte, installed updater.Installed) {
		plan, err := updater.PlanInstall(m, keys.PublicKey, artifact, installed)
		switch {
		case errors.Is(err, updater.ErrNotNewer):
			fmt.Println(label + ": refused, not newer")
		case errors.Is(err, updater.ErrWrongApp):
			fmt.Println(label + ": refused, wrong app")
		case errors.Is(err, updater.ErrDigestMismatch):
			fmt.Println(label + ": refused, tampered artifact")
		case err != nil:
			fmt.Println(label+": refused,", err)
		default:
			fmt.Println(label+": install", plan.Version)
		}
	}
	stable := func(app, version string) updater.Installed {
		return updater.Installed{AppID: app, Channel: updater.ChannelStable, Version: version}
	}
	check("upgrade", artifact, stable("com.example.notes", "1.2.0"))
	check("replay", artifact, stable("com.example.notes", "1.3.0"))
	check("other app", artifact, stable("com.example.mail", "1.0.0"))
	check("tampered", []byte("something else"), stable("com.example.notes", "1.2.0"))
	// Output:
	// upgrade: install 1.3.0
	// replay: refused, not newer
	// other app: refused, wrong app
	// tampered: refused, tampered artifact
}

// Versions compare by SemVer 2.0 precedence.
func ExampleCompareVersions() {
	for _, pair := range [][2]string{{"1.10.0", "1.9.0"}, {"1.0.0-rc.1", "1.0.0"}, {"2.0.0+build.5", "2.0.0"}} {
		c, _ := updater.CompareVersions(pair[0], pair[1])
		fmt.Println(pair[0], "vs", pair[1], "=", c)
	}
	// Output:
	// 1.10.0 vs 1.9.0 = 1
	// 1.0.0-rc.1 vs 1.0.0 = -1
	// 2.0.0+build.5 vs 2.0.0 = 0
}

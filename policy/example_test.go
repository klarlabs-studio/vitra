package policy_test

import (
	"context"
	"fmt"
	"strings"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/policy"
)

// An administrator's policy can take a permission away from an app that
// grants it. Policy only ever tightens.
func ExampleEngine() {
	doc, err := policy.ParseDocument(strings.NewReader(`{"schema": "1", "deny_permissions": ["clipboard.read"]}`))
	if err != nil {
		panic(err)
	}
	eng, err := policy.NewEngine(doc, policy.EnvProduction)
	if err != nil {
		panic(err)
	}

	rt, _ := vitra.New(vitra.Config{AppID: "com.example.notes"})
	_, _ = rt.OpenWindow(context.Background(), "main", domain.OriginPackagedLocal)
	grant, _ := domain.NewCapabilityGrant("main", "clipboard",
		[]domain.WindowID{"main"}, []domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "clipboard.read"}, {Name: "clipboard.write"}})
	_ = rt.RegisterGrant(grant)
	rt.SetPolicy(eng)

	caller, _ := rt.CallerFor("main")
	for _, p := range []domain.PermissionName{"clipboard.write", "clipboard.read"} {
		d := rt.Authorize(caller, p, "")
		fmt.Println(p, d.Allowed)
	}
	// Output:
	// clipboard.write true
	// clipboard.read false
}

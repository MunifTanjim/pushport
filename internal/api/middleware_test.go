package api

import (
	"crypto/rand"
	"net/http/httptest"
	"testing"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/crypto"
	"github.com/MunifTanjim/pushport/internal/db"
)

func newAppSvc(t *testing.T) (*app.Service, *db.DB) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	return app.NewService(d, kr), d
}

func TestAuthorizeApp(t *testing.T) {
	apps, _ := newAppSvc(t)

	call := func(pathAppID, hdr string) (authed, present bool, appID string) {
		req := httptest.NewRequest("POST", "/apps/"+pathAppID+"/instances", nil)
		req.SetPathValue("app_id", pathAppID)
		if hdr != "" {
			req.Header.Set("Authorization", hdr)
		}
		return authorizeApp(req, "admintok", apps)
	}

	if a, p, _ := call("x", ""); a || p {
		t.Fatalf("no token: want (false,false) got (%v,%v)", a, p)
	}
	if a, p, id := call("x", "Bearer admintok"); !a || !p || id != "x" {
		t.Fatalf("admin token: want (true,true,x) got (%v,%v,%q)", a, p, id)
	}
	if a, p, _ := call("x", "Bearer wrong"); a || !p {
		t.Fatalf("bad token: want (false,true) got (%v,%v)", a, p)
	}
	if a, p, _ := call("@app", "Bearer admintok"); a || !p {
		t.Fatalf("admin @app: want (false,true) got (%v,%v)", a, p)
	}
}

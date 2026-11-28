package reqguard

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/AICSbench/planetlogi/apps/api/internal/modules/auth"
)

// Regression: an unexported embedded struct is skipped by Huma, which silently dropped the
// cookie, Authorization header and locale for every Phase 2 request.
func TestCredentialsBindWhenEmbedded(t *testing.T) {
	_, api := humatest.New(t)
	var got Credentials
	huma.Register(api, huma.Operation{OperationID: "probe", Method: http.MethodGet, Path: "/probe"},
		func(_ context.Context, in *struct {
			Credentials
			Grade int `query:"grade"`
		}) (*struct{}, error) {
			got = in.Credentials
			return nil, nil
		})

	api.Get("/probe?locale=id&grade=3", "Authorization: Bearer abc", "Cookie: pl_access=tok")

	if got.CookieAccess != "tok" || got.Authorization != "Bearer abc" || got.Locale != "id" {
		t.Fatalf("credentials not bound: %+v", got)
	}
	params := api.OpenAPI().Paths["/probe"].Get.Parameters
	names := map[string]bool{}
	for _, p := range params {
		names[p.Name] = true
	}
	for _, want := range []string{"pl_access", "Authorization", "locale", "grade"} {
		if !names[want] {
			t.Fatalf("OpenAPI is missing parameter %q (have %v)", want, names)
		}
	}
}

type fakeResolver map[string]auth.Principal

func (f fakeResolver) ResolvePrincipal(_ context.Context, tok string) (*auth.Principal, error) {
	if p, ok := f[tok]; ok {
		return &p, nil
	}
	return nil, auth.ErrInvalidCredentials
}

func TestResolve(t *testing.T) {
	ctx := context.Background()
	r := fakeResolver{"good": {ID: "stu-1", Role: auth.RoleStudent}}

	cases := []struct {
		name        string
		creds       Credentials
		requireAuth bool
		wantID      string
		wantErr     bool
	}{
		{"cookie token", Credentials{CookieAccess: "good"}, true, "stu-1", false},
		{"bearer header, any case", Credentials{Authorization: "bEaReR good"}, true, "stu-1", false},
		{"non-bearer header ignored", Credentials{Authorization: "Basic good"}, true, "", true},
		{"invalid token never downgrades to guest", Credentials{CookieAccess: "expired"}, false, "", true},
		{"no token, auth required", Credentials{}, true, "", true},
		{"no token, read-only guest", Credentials{}, false, DemoStudentID, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Resolve(ctx, r, tc.creds, tc.requireAuth)
			if tc.wantErr {
				if !errors.Is(err, ErrUnauthenticated) {
					t.Fatalf("expected ErrUnauthenticated, got %v (%+v)", err, p)
				}
				return
			}
			if err != nil || p.ID != tc.wantID {
				t.Fatalf("got %+v err=%v, want %s", p, err, tc.wantID)
			}
		})
	}

	if _, err := Resolve(ctx, nil, Credentials{CookieAccess: "good"}, false); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("a token without a resolver must be rejected, got %v", err)
	}
}

func TestCopyAnswersBounds(t *testing.T) {
	src := map[string]string{"answer": "12"}
	cp, err := CopyAnswers(src)
	if err != nil || cp["answer"] != "12" {
		t.Fatalf("valid answers rejected: %v", err)
	}
	src["answer"] = "changed"
	if cp["answer"] != "12" {
		t.Fatal("CopyAnswers must not alias the input map")
	}

	tooMany := map[string]string{}
	for i := 0; i <= MaxAnswerParts; i++ {
		tooMany[strings.Repeat("k", i+1)] = "1"
	}
	for name, in := range map[string]map[string]string{
		"too many parts": tooMany,
		"long value":     {"answer": strings.Repeat("9", MaxAnswerValueLen+1)},
		"long key":       {strings.Repeat("k", MaxAnswerKeyLen+1): "1"},
		"empty key":      {"": "1"},
	} {
		if _, err := CopyAnswers(in); !errors.Is(err, ErrInvalidAnswers) {
			t.Fatalf("%s: expected ErrInvalidAnswers, got %v", name, err)
		}
	}
}

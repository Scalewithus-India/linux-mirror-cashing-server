package upstream

import "testing"

func TestNormalizePath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/ubuntu/pool/a.deb", "/ubuntu/pool/a.deb"},
		{"/ubuntu//pool/a.deb", "/ubuntu/pool/a.deb"},
		{"/ubuntu/pool/", "/ubuntu/pool/"},
		{"/ubuntu/foo/../bar", ""},
		{"/ubuntu/%2e%2e/bar", ""},
		{"/ubuntu/./x", ""},
	}
	for _, c := range cases {
		got := NormalizePath(c.in)
		if got != c.want {
			t.Fatalf("NormalizePath(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestResolveKey(t *testing.T) {
	r := Resolve("/ubuntu/dists/jammy/InRelease")
	if r == nil || r.Key != "ubuntu/dists/jammy/InRelease" {
		t.Fatalf("unexpected resolve: %+v", r)
	}
	if !IsMetadataKey(r.Key) {
		t.Fatal("expected metadata key")
	}
	if ResponseContentType("ubuntu/pool/a.deb", "text/html") != "application/octet-stream" {
		t.Fatal("package ctype")
	}
}

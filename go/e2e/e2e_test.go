//go:build e2e

package e2e

import (
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestE2E_RateLimitReturns429 builds the real server binary, seeds a temporary
// database, runs the server as a process, and drives it over a real HTTP socket.
// This is the only test that exercises main.go wiring (the real 10/min leaky
// bucket built via the factory), real SQLite, and a real network round-trip.
func TestE2E_RateLimitReturns429(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	moduleDir := filepath.Dir(wd) // go/e2e -> go

	dir := t.TempDir()
	bin := filepath.Join(dir, "server")

	// Build the server binary.
	build := exec.Command("go", "build", "-o", bin, "./cmd/server")
	build.Dir = moduleDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	// Seed a small database (the server refuses to serve an empty DB).
	seed := exec.Command(bin, "--seed", "--data", dir, "--contacts", "5", "--files", "0")
	if out, err := seed.CombinedOutput(); err != nil {
		t.Fatalf("seed: %v\n%s", err, out)
	}

	addr := freeAddr(t)
	srv := exec.Command(bin, "--data", dir, "--addr", addr)
	srv.Stdout, srv.Stderr = os.Stderr, os.Stderr
	if err := srv.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Process.Kill()
		_, _ = srv.Process.Wait()
	})

	base := "http://" + addr
	waitReady(t, base+"/health")

	// Fire a burst: the first 10 are admitted, the 11th is rejected with 429 and a
	// Retry-After header — no waiting needed, the burst is instantaneous.
	client := &http.Client{Timeout: 3 * time.Second}
	for i := 1; i <= 10; i++ {
		code, _ := get(t, client, base+"/api/contacts")
		if code == http.StatusTooManyRequests {
			t.Fatalf("request %d: unexpected 429 before the limit", i)
		}
	}
	code, retryAfter := get(t, client, base+"/api/contacts")
	if code != http.StatusTooManyRequests {
		t.Fatalf("11th request: want 429, got %d", code)
	}
	if retryAfter == "" {
		t.Fatal("11th request: 429 missing Retry-After header")
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func waitReady(t *testing.T, healthURL string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("server did not become ready within 10s")
}

func get(t *testing.T, client *http.Client, url string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer test@example.com")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Retry-After")
}

package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestInitializedProcessRequiresKeysAndCanLogIn(t *testing.T) {
	dir := t.TempDir()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	serverBin := filepath.Join(dir, "relay"+suffix)
	adminBin := filepath.Join(dir, "relay-admin"+suffix)
	build := func(path, output string) {
		t.Helper()
		cmd := exec.Command("go", "build", "-o", output, path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v: %s", path, err, out)
		}
	}
	build(".", serverBin)
	build("../relay-admin", adminBin)
	keyPath := filepath.Join(dir, "keys.json")
	dbPath := filepath.Join(dir, "relay.db")
	passwordPath := filepath.Join(dir, "password.txt")
	password := "SyntheticOnly-Owner-Password-2026"
	if err := os.WriteFile(passwordPath, []byte(password), 0o600); err != nil {
		t.Fatal(err)
	}
	runAdmin := func(args ...string) {
		t.Helper()
		cmd := exec.Command(adminBin, args...)
		if err := cmd.Run(); err != nil {
			t.Fatalf("admin command failed: %v", err)
		}
	}
	runAdmin("keygen", "-out", keyPath)
	runAdmin("bootstrap", "-database", dbPath, "-key-file", keyPath, "-password-file", passwordPath)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	origin := "https://" + address
	baseEnv := append(os.Environ(),
		"RELAY_DATABASE_PATH="+dbPath, "RELAY_HTTP_ADDR="+address,
		"RELAY_MASTER_KEY_FILE=", "RELAY_PUBLIC_ORIGIN=")
	missing := exec.Command(serverBin)
	missing.Env = baseEnv
	if err := missing.Run(); err == nil {
		t.Fatal("initialized server started without key and HTTPS origin")
	}
	server := exec.Command(serverBin)
	server.Env = append(baseEnv,
		"RELAY_MASTER_KEY_FILE="+keyPath, "RELAY_PUBLIC_ORIGIN="+origin)
	var logs bytes.Buffer
	server.Stdout = &logs
	server.Stderr = &logs
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		server.Process.Kill()
		server.Wait()
	}()
	client := &http.Client{Timeout: time.Second}
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		resp, err := client.Get("http://" + address + "/readyz")
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("initialized server did not become ready: %s", logs.String())
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+address+"/v1/session",
		strings.NewReader(fmt.Sprintf(`{"password":%q}`, password)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("owner login status = %d", resp.StatusCode)
	}
	if len(resp.Cookies()) != 1 || resp.Cookies()[0].Name != "__Host-relay-session" ||
		!resp.Cookies()[0].Secure || !resp.Cookies()[0].HttpOnly {
		t.Fatal("owner login did not set a secure session cookie")
	}
}

package providers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lazypass/internal/config"
	"lazypass/internal/vault"
	ageprovider "lazypass/internal/vault/age"
	"lazypass/internal/vault/metadata"
)

func TestCrossMachineMetadata(t *testing.T) {
	if remote := os.Getenv("LAZYPASS_METADATA_REMOTE_ROOT"); remote != "" {
		for _, name := range []string{"age", "pass"} {
			p, err := Builtins().Open(config.Vault{Name: name, Provider: name, StoreDir: filepath.Join(remote, "fixtures", name), IdentityFile: "/nonexistent-fixture-identity"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			nodes, err := p.List(context.Background(), nil)
			if err != nil || len(nodes) != 5 {
				t.Fatalf("%s listing: %v", name, err)
			}
			for _, node := range nodes {
				if !node.TypeKnown || node.Name != node.EntryType.String() {
					t.Fatalf("%s type missing: %s", name, node.Name)
				}
			}
			if os.Getenv("LAZYPASS_METADATA_REMOTE_EDIT") == "1" {
				ctx := context.Background()
				if name == "age" {
					if err := p.Write(ctx, vault.Entry{Path: vault.Path{"Identity"}, Type: vault.IdentityEntry, Notes: "public remote edit", Fields: map[string]string{"firstName": "Remote"}}); err != nil {
						t.Fatal(err)
					}
					if err := p.Delete(ctx, vault.Path{"Card"}); err != nil {
						t.Fatal(err)
					}
				} else {
					store := p.StoreDirectory()
					if err := os.Remove(filepath.Join(store, "Card.gpg")); err != nil {
						t.Fatal(err)
					}
					if err := metadata.Remove(store, vault.Path{"Card"}, false); err != nil {
						t.Fatal(err)
					}
					if err := metadata.Write(store, vault.Path{"Identity"}, vault.IdentityEntry); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		return
	}
	host := os.Getenv("LAZYPASS_METADATA_SSH_HOST")
	if host == "" {
		t.Skip("set LAZYPASS_METADATA_SSH_HOST")
	}
	local := t.TempDir()
	fixtures := filepath.Join(local, "fixtures")
	ageStore := filepath.Join(fixtures, "age")
	if err := ageprovider.Initialize(ageStore, filepath.Join(local, "identity.age"), "public-fixture-passphrase"); err != nil {
		t.Fatal(err)
	}
	age := ageprovider.New(ageStore, filepath.Join(local, "identity.age"), nil)
	passStore := filepath.Join(fixtures, "pass")
	if err := os.MkdirAll(passStore, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(passStore, ".gpg-id"), []byte("public-metadata-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range vault.AllEntryTypes() {
		path := vault.Path{kind.String()}
		if err := age.Create(context.Background(), vault.Entry{Path: path, Type: kind, Password: "public-fixture", Fields: map[string]string{"firstName": "Fixture"}}); err != nil {
			t.Fatal(err)
		}
		// pass listing only needs ciphertext filenames; no private key is copied.
		if err := os.WriteFile(filepath.Join(passStore, kind.String()+".gpg"), []byte("opaque-fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := metadata.Write(passStore, path, kind); err != nil {
			t.Fatal(err)
		}
	}
	command := func(name string, args ...string) []byte {
		t.Helper()
		output, err := exec.Command(name, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s failed: %v %s", name, err, output)
		}
		return output
	}
	remote := strings.TrimSpace(string(command("ssh", "-o", "BatchMode=yes", host, "mktemp -d /tmp/lazypass-metadata-test.XXXXXXXX")))
	if !strings.HasPrefix(remote, "/tmp/lazypass-metadata-test.") || strings.ContainsAny(remote, "\n '\"") {
		t.Fatal("invalid remote fixture directory")
	}
	t.Cleanup(func() {
		if output, err := exec.Command("ssh", "-o", "BatchMode=yes", host, "rm -rf -- "+remote).CombinedOutput(); err != nil {
			t.Errorf("remote cleanup: %v %s", err, output)
		}
	})
	binary := filepath.Join(local, "metadata.test")
	command("go", "test", "-c", "-o", binary, "lazypass/internal/vault/providers")
	command("scp", "-qr", fixtures, binary, host+":"+remote+"/")
	output := command("ssh", "-o", "BatchMode=yes", host, "LAZYPASS_METADATA_REMOTE_ROOT="+remote+" LAZYPASS_METADATA_REMOTE_EDIT=1 "+remote+"/metadata.test -test.run '^TestCrossMachineMetadata$' -test.v")
	t.Log(strings.TrimSpace(string(output)))
	returned := filepath.Join(local, "returned")
	if err := os.Mkdir(returned, 0700); err != nil {
		t.Fatal(err)
	}
	command("scp", "-qr", host+":"+remote+"/fixtures", returned+"/")
	for _, name := range []string{"age", "pass"} {
		p, err := Builtins().Open(config.Vault{Name: name, Provider: name, StoreDir: filepath.Join(returned, "fixtures", name), IdentityFile: filepath.Join(local, "identity.age")}, func() (string, error) { return "public-fixture-passphrase", nil })
		if err != nil {
			t.Fatal(err)
		}
		nodes, err := p.List(context.Background(), nil)
		if err != nil || len(nodes) != 4 {
			t.Fatalf("returned %s deletion: %v", name, err)
		}
		for _, node := range nodes {
			if !node.TypeKnown || node.EntryType == vault.CardEntry {
				t.Fatal("returned metadata stale")
			}
		}
		if name == "age" {
			entry, err := p.Read(context.Background(), vault.Path{"Identity"})
			if err != nil || entry.Notes != "public remote edit" || entry.Fields["firstName"] != "Remote" {
				t.Fatalf("remote encrypted edit not restored: %v", err)
			}
		}
	}
}

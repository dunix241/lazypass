package pass

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lazypass/internal/clipboard"
	"lazypass/internal/vault"
)

// This suite uses only an ephemeral key and public fixtures.
func TestEphemeralVaultIntegration(t *testing.T) {
	if os.Getenv("LAZYPASS_INTEGRATION") != "1" {
		t.Skip("set LAZYPASS_INTEGRATION=1 to run real pass/GPG tests")
	}
	for _, tool := range []string{"pass", "gpg", "gpgconf"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " unavailable")
		}
	}
	home := t.TempDir()
	t.Setenv("GNUPGHOME", home)
	command := func(name string, args ...string) []byte {
		t.Helper()
		cmd := exec.Command(name, args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s failed: %v: %s", name, err, output)
		}
		return output
	}
	defer exec.Command("gpgconf", "--kill", "gpg-agent").Run()
	// A protocol fixture answers pinentry with a public test passphrase.
	pinentry := filepath.Join(home, "pinentry-fixture")
	script := "#!/bin/sh\nprintf 'OK fixture ready\\n'\nwhile IFS= read -r line; do\ncase \"$line\" in\nGETPIN*) printf 'D public-fixture-passphrase\\nOK\\n';;\nBYE*) printf 'OK\\n'; exit 0;;\n*) printf 'OK\\n';;\nesac\ndone\n"
	if err := os.WriteFile(pinentry, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "gpg-agent.conf"), []byte("pinentry-program "+pinentry+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase-fd", "0", "--quick-generate-key", "lazypass fixture <fixture@example.invalid>", "default", "default", "1d")
	cmd.Stdin = strings.NewReader("public-fixture-passphrase\n")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("key generation: %v %s", err, output)
	}
	listing := string(command("gpg", "--batch", "--with-colons", "--list-secret-keys"))
	fingerprint := ""
	for _, line := range strings.Split(listing, "\n") {
		parts := strings.Split(line, ":")
		if len(parts) > 9 && parts[0] == "fpr" {
			fingerprint = parts[9]
			break
		}
	}
	if fingerprint == "" {
		t.Fatal("fixture key missing")
	}
	root := filepath.Join(home, "store")
	t.Setenv("PASSWORD_STORE_DIR", root)
	command("pass", "init", fingerprint)
	t.Setenv("GIT_AUTHOR_NAME", "lazypass fixture")
	t.Setenv("GIT_AUTHOR_EMAIL", "fixture@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "lazypass fixture")
	t.Setenv("GIT_COMMITTER_EMAIL", "fixture@example.invalid")
	command("pass", "git", "init")
	p := New(root, nil)
	ctx := context.Background()
	path := vault.Path{"hello", "world"}
	if err := p.Create(ctx, vault.Entry{Path: path, Password: "public-vault-fixture"}); err != nil {
		t.Fatal(err)
	}
	tracked := string(command("git", "-C", root, "ls-files"))
	if strings.Contains(tracked, ".lazypass-create-") || !strings.Contains(tracked, "hello/world.gpg") {
		t.Fatalf("invalid staged files: %s", tracked)
	}
	if err := p.Create(ctx, vault.Entry{Path: path, Password: "must-not-replace"}); !errors.Is(err, vault.ErrConflict) {
		t.Fatalf("duplicate creation: %v", err)
	}
	command("gpgconf", "--kill", "gpg-agent")
	if _, err := p.Read(vault.WithoutPrompt(ctx), path); !errors.Is(err, vault.ErrPromptRequired) {
		t.Fatalf("expired cache should require prompt: %v", err)
	}
	entry, err := p.Read(ctx, path)
	if err != nil || entry.Password != "public-vault-fixture" {
		t.Fatalf("pinentry retry: %v", err)
	}
	entry.Password = "public-edited-fixture"
	if err := p.Write(ctx, entry); err != nil {
		t.Fatal(err)
	}
	entry, err = p.Read(vault.WithoutPrompt(ctx), path)
	if err != nil || entry.Password != "public-edited-fixture" {
		t.Fatalf("cached edited value: %v", err)
	}
	if os.Getenv("LAZYPASS_INTEGRATION_CLIPBOARD") == "1" {
		if err := clipboard.Copy(entry.Password); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("authenticator encrypted round trip", func(t *testing.T) {
		const uri = "otpauth://totp/Fixture?secret=JBSWY3DPEHPK3PXP"
		entry.Authenticator = uri
		if err := p.Write(ctx, entry); err != nil {
			t.Fatal(err)
		}
		output, err := exec.Command("pass", "show", path.String()).Output()
		if err != nil {
			t.Fatal(err)
		}
		text := string(output)
		if !strings.HasPrefix(text, entry.Password+"\n") || !strings.Contains(text, "\n"+uri) {
			t.Fatal("decrypted entry is not pass-otp compatible")
		}
		loaded, err := p.Read(ctx, path)
		if err != nil || loaded.Authenticator != uri {
			t.Fatal("encrypted authenticator did not round trip")
		}
		loaded.Authenticator = ""
		if err := p.Write(ctx, loaded); err != nil {
			t.Fatal(err)
		}
		text = string(command("pass", "show", path.String()))
		if strings.Contains(text, "otpauth") {
			t.Fatal("removed authenticator remains in encrypted entry")
		}
	})
	if err := p.Delete(ctx, path); err != nil {
		t.Fatal(err)
	}
	if nodes, err := p.List(ctx, vault.Path{"hello"}); err != nil || len(nodes) != 0 {
		t.Fatalf("last-child parent preservation: %v %v", nodes, err)
	}
	if err := p.Create(ctx, vault.Entry{Path: vault.Path{"hello", "nested", "child"}, Password: "public-fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := p.DeleteFolder(ctx, vault.Path{"hello"}); err != nil {
		t.Fatal(err)
	}
	if nodes, err := p.List(ctx, nil); err != nil || len(nodes) != 0 {
		t.Fatalf("recursive delete: %v %v", nodes, err)
	}
	if os.Getenv("LAZYPASS_INTEGRATION_TUI") == "1" {
		testVaultTerminal(t, home, root, p)
	}
	command("gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase", "", "--quick-generate-key", "removal destination <destination@example.invalid>", "rsa2048", "encr", "1d")
	destinationKey := ""
	for _, line := range strings.Split(string(command("gpg", "--batch", "--with-colons", "--list-keys", "destination@example.invalid")), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) > 9 && parts[0] == "fpr" {
			destinationKey = parts[9]
			break
		}
	}
	if destinationKey == "" {
		t.Fatal("destination key missing")
	}
	command("pass", "init", "--path=Unorganized", destinationKey)
	command("pass", "init", "--path=removal", fingerprint)
	for _, path := range []vault.Path{{"removal", "entry"}, {"removal", "nested", "child"}} {
		if err := p.Create(ctx, vault.Entry{Path: path, Password: "public-preserved-fixture"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.RemoveFolder(ctx, vault.Path{"removal"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []vault.Path{{"Unorganized", "entry"}, {"Unorganized", "nested", "child"}} {
		entry, err := p.Read(ctx, path)
		if err != nil || entry.Password != "public-preserved-fixture" {
			t.Fatalf("preserving removal: %v", err)
		}
		packets := strings.ToUpper(string(command("gpg", "--batch", "--list-only", "--list-packets", filepath.Join(root, path.String()+".gpg"))))
		if !strings.Contains(packets, strings.ToUpper(destinationKey[len(destinationKey)-16:])) {
			t.Fatal("moved ciphertext does not target destination recipient")
		}
	}
	if dirty := string(command("git", "-C", root, "status", "--porcelain")); dirty != "" {
		t.Fatalf("removal left Git bookkeeping dirty: %s", dirty)
	}
}

func testVaultTerminal(t *testing.T, home, root string, p *Provider) {
	t.Helper()
	binary := filepath.Join(home, "lazypass")
	if output, err := exec.Command("go", "build", "-o", binary, "lazypass").CombinedOutput(); err != nil {
		t.Fatalf("build TUI: %v %s", err, output)
	}
	config := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(config, []byte("vaults:\n  - name: fixture\n    provider: pass\n    storeDir: "+root+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	drive := func(keys string) {
		t.Helper()
		// The PTY is isolated from the user's terminal; only public fixture values
		// are rendered. Check terminal mode restoration on both exit paths.
		script := `import os,pty,subprocess,sys,termios,time,fcntl,struct,select,codecs,pyte
master,slave=pty.openpty()
fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack("HHHH",24,84,0,0))
before=termios.tcgetattr(slave)
env=dict(os.environ,TERM="xterm-256color",SSH_CONNECTION="fixture")
def terminal_session():
 os.setsid();fcntl.ioctl(slave,termios.TIOCSCTTY,0)
p=subprocess.Popen([sys.argv[1],"--config",sys.argv[2],"vault"],stdin=slave,stdout=slave,stderr=slave,env=env,preexec_fn=terminal_session)
captured=bytearray()
screen=pyte.Screen(84,24)
stream=pyte.Stream(screen)
decoder=codecs.getincrementaldecoder("utf-8")("replace")
def drain(delay):
 end=time.time()+delay
 while time.time()<end:
  ready,_,_=select.select([master],[],[],.05)
  if ready:
   data=os.read(master,65536)
   captured.extend(data);stream.feed(decoder.decode(data))
def wait_ui(expected):
 deadline=time.time()+20
 while time.time()<deadline:
  text="\n".join(screen.display)
  if expected in text and "Loading" not in text: return
  drain(.05)
 raise AssertionError("UI did not reach "+repr(expected)+": "+repr("\n".join(screen.display)))
wait_ui("No entries in this folder." if not os.path.isdir(os.path.join(os.environ["PASSWORD_STORE_DIR"],"hello")) else "hello/")
for key in sys.argv[3].split("|"):
 if key.startswith("WAITUI:"):
  wait_ui(key[len("WAITUI:"):])
 elif key.startswith("ABSENT:"):
  assert key[len("ABSENT:"):] not in "\n".join(screen.display),"masked field exposed fixture"
 elif key.startswith("RESIZE:"):
  width,height=map(int,key[len("RESIZE:"):].split("x"))
  fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack("HHHH",height,width,0,0))
  screen.resize(height,width);p.send_signal(__import__("signal").SIGWINCH);drain(.2)
 elif key=="CHECKOTP":
  import base64,re
  values=re.findall(rb"\x1b\]52;c;([A-Za-z0-9+/=]+)(?:\x07|\x1b\\)",bytes(captured))
  assert values,"OSC52 copy missing"
  value=base64.b64decode(values[-1])
  assert len(value)==6 and value.isdigit(),"OTP clipboard payload invalid"
 elif key.startswith("WAITABSENT:"):
  filename=key[len("WAITABSENT:"):];deadline=time.time()+20
  while os.path.exists(filename) and time.time()<deadline: drain(.05)
  assert not os.path.exists(filename),"delete did not complete"
 elif key.startswith("WAITFILE:"):
  filename=key[len("WAITFILE:"):];deadline=time.time()+15
  while not os.path.isfile(filename) and time.time()<deadline: drain(.1)
  assert os.path.isfile(filename),"save did not complete: "+repr(bytes(captured[-12000:]))
 elif key.startswith("WAITTEXT:"):
  filename,expected=key[len("WAITTEXT:"):].split("=",1);deadline=time.time()+15
  while time.time()<deadline:
   result=subprocess.run(["pass","show","--",filename],env=env,capture_output=True)
   if result.returncode==0 and result.stdout.decode().splitlines()[0]==expected: break
   drain(.1)
  else: raise AssertionError("edit did not complete")
 else:
  if len(key)>1: os.write(master,b"\x1b[200~"+key.encode()+b"\x1b[201~")
  else: os.write(master,key.encode())
  drain(.5 if key=="\x1b" else .05)
p.wait(timeout=10)
assert p.returncode==0,p.returncode
assert termios.tcgetattr(slave)==before,"terminal modes not restored"
os.close(master);os.close(slave)
`
		if output, err := exec.Command("python3", "-c", script, binary, config, keys).CombinedOutput(); err != nil {
			t.Fatalf("TUI fixture: %v %s", err, output)
		}
	}
	drive("a|WAITUI:Login|\r|WAITUI:Destination|hello/world|\r|WAITUI:Password:|public-tui-fixture|\r|WAITFILE:" + filepath.Join(root, "hello", "world.gpg") + "|WAITUI:Stored in vault|q")
	entry, err := p.Read(context.Background(), vault.Path{"hello", "world"})
	if err != nil || entry.Password != "public-tui-fixture" {
		t.Fatalf("TUI add failed: %v", err)
	}
	drive("l|WAITUI:world|\r|WAITUI:Copied to clipboard|e|WAITUI:Password:|\x15|public-tui-edited|\r|WAITTEXT:hello/world=public-tui-edited|WAITUI:Stored in vault|q")
	entry, err = p.Read(context.Background(), vault.Path{"hello", "world"})
	if err != nil || entry.Password != "public-tui-edited" {
		t.Fatalf("TUI search/edit failed: %v", err)
	}
	entry.Authenticator = "otpauth://totp/Fixture?secret=JBSWY3DPEHPK3PXP"
	if err := p.Write(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	drive("l|WAITUI:world|o|WAITUI:Copied to clipboard|CHECKOTP|e|WAITUI:Authenticator|ABSENT:JBSWY3DPEHPK3PXP|RESIZE:40x20|ABSENT:JBSWY3DPEHPK3PXP|RESIZE:84x24|\x1b|q")
	drive("l|WAITUI:world|d|WAITUI:Delete entry|n|d|WAITUI:Delete entry|y|WAITABSENT:" + filepath.Join(root, "hello", "world.gpg") + "|WAITUI:Deleted from vault|q")
	if nodes, err := p.List(context.Background(), vault.Path{"hello"}); err != nil || len(nodes) != 0 {
		t.Fatalf("TUI delete/parent preservation: %v %v", nodes, err)
	}
	drive("D|WAITUI:Delete folder|hello|\r|WAITABSENT:" + filepath.Join(root, "hello") + "|WAITUI:Deleted from vault|q")
	if nodes, err := p.List(context.Background(), nil); err != nil || len(nodes) != 0 {
		t.Fatalf("TUI folder delete: %v %v", nodes, err)
	}
}

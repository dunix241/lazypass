package age

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"lazypass/internal/vault"
)

func TestAgeTerminalWorkflow(t *testing.T) {
	if os.Getenv("LAZYPASS_INTEGRATION_TUI") != "1" {
		t.Skip("set LAZYPASS_INTEGRATION_TUI=1 for real terminal qualification")
	}
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	identity := filepath.Join(dir, "identity")
	if err := Initialize(store, identity, "public-age-fixture"); err != nil {
		t.Fatal(err)
	}
	p := New(store, identity, func() (string, error) { return "public-age-fixture", nil })
	defer p.Lock()
	ctx := context.Background()
	if err := p.Create(ctx, vault.Entry{Path: vault.Path{"hello"}, Password: "public-value"}); err != nil {
		t.Fatal(err)
	}
	if err := p.Create(ctx, vault.Entry{Path: vault.Path{"hello", "child"}, Password: "public-child"}); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(config, []byte("vaults:\n  - name: age-fixture\n    provider: age\n    storeDir: "+store+"\n    identityFile: "+identity+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "lazypass")
	if output, err := exec.Command("go", "build", "-o", binary, "lazypass").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	script := `import os,pty,subprocess,sys,termios,time,fcntl,struct,select,codecs,pyte
master,slave=pty.openpty()
fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack("HHHH",24,84,0,0))
before=termios.tcgetattr(slave)
def session():
 os.setsid();fcntl.ioctl(slave,termios.TIOCSCTTY,0)
p=subprocess.Popen([sys.argv[1],"--config",sys.argv[2],"vault"],stdin=slave,stdout=slave,stderr=slave,env=dict(os.environ,TERM="xterm-256color",SSH_CONNECTION="fixture"),preexec_fn=session)
screen=pyte.Screen(84,24);stream=pyte.Stream(screen);decoder=codecs.getincrementaldecoder("utf-8")("replace")
def drain(delay=.15):
 end=time.time()+delay
 while time.time()<end:
  if select.select([master],[],[],.03)[0]: stream.feed(decoder.decode(os.read(master,65536)))
def wait(text):
 deadline=time.time()+20
 while time.time()<deadline:
  if text in "\n".join(screen.display): return
  drain()
 raise AssertionError("UI missing "+text+": "+repr(screen.display))
def key(text):
 os.write(master,text.encode());drain(.5 if text=="\x1b" else .15)
def paste(text):
 key("\x1b[200~"+text+"\x1b[201~")
def wait_file(name):
 deadline=time.time()+20
 while time.time()<deadline:
  if os.path.isfile(os.path.join(sys.argv[3],name)): return
  drain()
 raise AssertionError("file missing: "+name)
try:
 wait("hello/")
 wait("Unlock vault");paste("public-age-fixture");key("\r");drain(.5)
 key("y");wait("(copy)")
 assert sum(row.count("(copy)") for row in screen.display)==1,"same-name folder and entry share staging"
 key("\x1b");assert "(copy)" not in "\n".join(screen.display)
 key("m");key("y");key("\x1b")
 assert "marked" not in "\n".join(screen.display) and "(copy)" not in "\n".join(screen.display)
 key("y");key("p");wait("Paste conflict");key("r");key("\x15");paste("folder-copy");key("\r");wait_file("folder-copy/child.age")
 wait("Pasted 1 item");key("\x1b")
 key("j");key("j");key("x");key("k");key("l");wait("age-fixture/hello");key("p");wait_file("hello/hello.age");wait("Pasted 1 item")
 key("c");wait("Copied to clipboard")
 key("r");wait("Name");key("\x15");paste("renamed");key("\r");wait_file("hello/renamed.age")
 wait("Renamed in vault")
 fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack("HHHH",20,40,0,0));screen.resize(20,40);p.send_signal(__import__("signal").SIGWINCH);drain(.3)
 key("?");wait("Keybindings");key("\x1b");key("q")
 p.wait(timeout=10)
 assert p.returncode==0 and termios.tcgetattr(slave)==before,"terminal not restored"
finally:
 if p.poll() is None: p.terminate();p.wait(timeout=10)
 os.close(master);os.close(slave)
`
	if output, err := exec.Command("python3", "-c", script, binary, config, store).CombinedOutput(); err != nil {
		t.Fatalf("age PTY: %v %s", err, output)
	}
	for _, path := range []vault.Path{{"folder-copy", "child"}, {"hello", "renamed"}} {
		entry, err := p.Read(ctx, path)
		if err != nil || entry.Password == "" {
			t.Fatal("transferred age fixture did not decrypt")
		}
	}
}

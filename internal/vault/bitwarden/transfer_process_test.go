package bitwarden

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

// The helper is a separate OS process so killing it exercises kernel lock
// release and persisted state, rather than cancellation in the same process.
func TestTransferProcessHelper(t *testing.T) {
	if os.Getenv("LAZYPASS_PROCESS_HELPER") != "1" {
		return
	}
	dir := os.Getenv("LAZYPASS_PROCESS_DIR")
	lock := flock.New(filepath.Join(dir, "lazypass-transfers.lock"))
	if err := lock.Lock(); err != nil {
		os.Exit(2)
	}
	p := New(dir, "", nil, nil)
	var signature [32]byte
	signature[0] = 1
	p.cloneAttempts = map[[32]byte]*cloneAttempt{signature: {SourceID: "fixture-source", TargetID: "confirmed-destination", BeforeIDs: []string{"original"}}}
	if p.saveCloneJournal("fixture-account") != nil {
		os.Exit(3)
	}
	if _, err := os.Stdout.WriteString("ready\n"); err != nil {
		os.Exit(4)
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestProcessCrashReleasesLockAndRetainsTransferReceipt(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestTransferProcessHelper$")
	cmd.Env = append(os.Environ(), "LAZYPASS_PROCESS_HELPER=1", "LAZYPASS_PROCESS_DIR="+dir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	ready := make(chan bool, 1)
	go func() { scanner := bufio.NewScanner(stdout); ready <- scanner.Scan() && scanner.Text() == "ready" }()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("helper failed before persistence")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("helper startup timed out")
	}
	contender := flock.New(filepath.Join(dir, "lazypass-transfers.lock"))
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	locked, err := contender.TryLockContext(ctx, 10*time.Millisecond)
	cancel()
	if locked {
		_ = contender.Unlock()
		t.Fatal("second process acquired held lock")
	}
	if err == nil {
		t.Fatal("contention did not report deadline")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	waited = true
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	locked, err = contender.TryLockContext(ctx, 10*time.Millisecond)
	if err != nil || !locked {
		t.Fatal("crash did not release kernel lock", err)
	}
	defer contender.Unlock()
	restarted := New(dir, "", nil, nil)
	restarted.cloneAttempts = make(map[[32]byte]*cloneAttempt)
	if err := restarted.loadCloneJournal("fixture-account"); err != nil {
		t.Fatal(err)
	}
	var signature [32]byte
	signature[0] = 1
	if attempt := restarted.cloneAttempts[signature]; attempt == nil || attempt.TargetID != "confirmed-destination" || attempt.Complete {
		t.Fatal("crash lost pending destination receipt")
	}
}

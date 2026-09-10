package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJournalPersistsHashChainAndReopens(t *testing.T) {
	dir := t.TempDir()
	j, err := OpenJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	first, err := j.Append(JournalRecord{ExecutionID: "exec-1", NodeID: "n1", Attempt: 1, State: "intent", CommandIntent: "cmd:90"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := j.Append(JournalRecord{ExecutionID: "exec-1", NodeID: "n1", Attempt: 1, State: "unknown", EvidenceRef: "timeout"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Seq != 1 || second.Seq != 2 || second.PrevDigest != first.Digest || second.Digest == "" {
		t.Fatalf("invalid chain: first=%#v second=%#v", first, second)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenJournal(dir)
	if err != nil {
		t.Fatalf("reopen journal: %v", err)
	}
	defer reopened.Close()
	third, err := reopened.Append(JournalRecord{ExecutionID: "exec-1", NodeID: "n1", Attempt: 1, State: "quarantined"})
	if err != nil {
		t.Fatal(err)
	}
	if third.Seq != 3 || third.PrevDigest != second.Digest {
		t.Fatalf("reopened chain did not continue: %#v", third)
	}
	if _, err := filepath.Abs(dir); err != nil {
		t.Fatal(err)
	}
}

func TestJournalRejectsTamperedRecord(t *testing.T) {
	dir := t.TempDir()
	j, err := OpenJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append(JournalRecord{ExecutionID: "exec-1", NodeID: "n1", State: "completed"}); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "execution-journal.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 20 {
		t.Fatal("journal fixture unexpectedly short")
	}
	data[len(data)-10] = 'x'
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenJournal(dir); err == nil {
		t.Fatal("tampered journal was accepted")
	}
}

func TestJournalRecoveryQuarantinesAmbiguousExecution(t *testing.T) {
	j, err := OpenJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if _, err := j.Append(JournalRecord{ExecutionID: "ambiguous", NodeID: "n1", State: "intent"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := j.RecoverExecution("ambiguous")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.CanResume || !snapshot.Quarantined || snapshot.LastRecord == nil {
		t.Fatalf("ambiguous recovery = %#v", snapshot)
	}
}

func TestJournalRecoveryAllowsCheckpoint(t *testing.T) {
	j, err := OpenJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if _, err := j.Append(JournalRecord{ExecutionID: "safe", NodeID: "n1", State: "checkpoint"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := j.RecoverExecution("safe")
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.CanResume || snapshot.Quarantined {
		t.Fatalf("checkpoint recovery = %#v", snapshot)
	}
}

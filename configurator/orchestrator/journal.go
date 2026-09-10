package orchestrator

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// JournalRecord is the durable fact written around one external-effect attempt.
// A record with state unknown is never replayed automatically.
type JournalRecord struct {
	Seq           uint64    `json:"seq"`
	ExecutionID   string    `json:"execution_id"`
	GraphRevision string    `json:"graph_revision"`
	NodeID        string    `json:"node_id"`
	Attempt       uint32    `json:"attempt"`
	InputDigest   string    `json:"input_digest,omitempty"`
	CommandIntent string    `json:"command_intent,omitempty"`
	Outcome       string    `json:"outcome,omitempty"`
	EvidenceRef   string    `json:"evidence_ref,omitempty"`
	FenceToken    string    `json:"fence_token,omitempty"`
	State         string    `json:"state"`
	Timestamp     time.Time `json:"timestamp"`
	PrevDigest    string    `json:"prev_digest,omitempty"`
	Digest        string    `json:"digest"`
}

var ErrJournalCorrupt = errors.New("durable journal is corrupt")

// Journal is a small append-only, hash-chained journal. It deliberately does
// not infer success from a missing or incomplete record.
type Journal struct {
	mu     sync.Mutex
	file   *os.File
	seq    uint64
	digest string
}

// RecoverySnapshot is the conservative recovery decision derived from facts.
// Unknown external effects are never considered resumable.
type RecoverySnapshot struct {
	ExecutionID string         `json:"execution_id"`
	LastRecord  *JournalRecord `json:"last_record,omitempty"`
	CanResume   bool           `json:"can_resume"`
	Quarantined bool           `json:"quarantined"`
	Reason      string         `json:"reason"`
}

func OpenJournal(dir string) (*Journal, error) {
	if dir == "" {
		return nil, fmt.Errorf("journal directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create journal directory: %w", err)
	}
	path := filepath.Join(dir, "execution-journal.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open execution journal: %w", err)
	}
	j := &Journal{file: f}
	if err := j.replayFile(); err != nil {
		_ = f.Close()
		return nil, err
	}
	return j, nil
}

func (j *Journal) replayFile() error {
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind execution journal: %w", err)
	}
	scanner := bufio.NewScanner(j.file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)
	var expectedSeq uint64 = 1
	prev := ""
	for scanner.Scan() {
		var record JournalRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return fmt.Errorf("%w: decode record %d: %v", ErrJournalCorrupt, expectedSeq, err)
		}
		if record.Seq != expectedSeq || record.PrevDigest != prev || record.Digest != digestRecord(record) {
			return fmt.Errorf("%w: chain mismatch at record %d", ErrJournalCorrupt, expectedSeq)
		}
		expectedSeq++
		prev = record.Digest
		j.seq, j.digest = record.Seq, record.Digest
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read execution journal: %w", err)
	}
	_, err := j.file.Seek(0, io.SeekEnd)
	return err
}

func (j *Journal) Append(record JournalRecord) (JournalRecord, error) {
	if j == nil || j.file == nil {
		return JournalRecord{}, fmt.Errorf("journal is not initialized")
	}
	if record.State == "" {
		return JournalRecord{}, fmt.Errorf("journal record state is required")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	record.Seq = j.seq + 1
	record.PrevDigest = j.digest
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}
	record.Digest = digestRecord(record)
	data, err := json.Marshal(record)
	if err != nil {
		return JournalRecord{}, fmt.Errorf("encode journal record: %w", err)
	}
	if _, err := j.file.Write(append(data, '\n')); err != nil {
		return JournalRecord{}, fmt.Errorf("append journal record: %w", err)
	}
	if err := j.file.Sync(); err != nil {
		return JournalRecord{}, fmt.Errorf("sync journal record: %w", err)
	}
	j.seq, j.digest = record.Seq, record.Digest
	return record, nil
}

// RecoverExecution reads one execution without mutating the journal. A
// checkpoint or completed node is resumable; intent/running/unknown states
// require quarantine because a physical effect may have happened.
func (j *Journal) RecoverExecution(executionID string) (RecoverySnapshot, error) {
	if j == nil || j.file == nil {
		return RecoverySnapshot{}, fmt.Errorf("journal is not initialized")
	}
	if executionID == "" {
		return RecoverySnapshot{}, fmt.Errorf("execution ID is required")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return RecoverySnapshot{}, fmt.Errorf("rewind execution journal: %w", err)
	}
	scanner := bufio.NewScanner(j.file)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	var expectedSeq uint64 = 1
	prev := ""
	var last *JournalRecord
	for scanner.Scan() {
		var record JournalRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return RecoverySnapshot{}, fmt.Errorf("%w: decode record %d: %v", ErrJournalCorrupt, expectedSeq, err)
		}
		if record.Seq != expectedSeq || record.PrevDigest != prev || record.Digest != digestRecord(record) {
			return RecoverySnapshot{}, fmt.Errorf("%w: chain mismatch at record %d", ErrJournalCorrupt, expectedSeq)
		}
		if record.ExecutionID == executionID {
			copy := record
			last = &copy
		}
		expectedSeq++
		prev = record.Digest
	}
	if err := scanner.Err(); err != nil {
		return RecoverySnapshot{}, fmt.Errorf("read execution journal: %w", err)
	}
	if _, err := j.file.Seek(0, io.SeekEnd); err != nil {
		return RecoverySnapshot{}, err
	}
	snapshot := RecoverySnapshot{ExecutionID: executionID, LastRecord: last}
	if last == nil {
		snapshot.Reason = "execution not found"
		return snapshot, nil
	}
	switch last.State {
	case "checkpoint", "completed":
		snapshot.CanResume = true
		snapshot.Reason = "last durable boundary is safe"
	default:
		snapshot.Quarantined = true
		snapshot.Reason = "last boundary may have an ambiguous external effect"
	}
	return snapshot, nil
}

func digestRecord(record JournalRecord) string {
	record.Digest = ""
	data, _ := json.Marshal(record)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (j *Journal) Close() error {
	if j == nil || j.file == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.file.Close()
}

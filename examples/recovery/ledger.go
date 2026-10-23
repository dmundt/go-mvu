package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"
)

const (
	frameSize  = 1024
	checksumAt = frameSize - sha256.Size
	maxEntries = 128
	maxBytes   = 2 * maxEntries * frameSize
	magic      = "GMVUWAL1"
)

var (
	ErrConflict  = errors.New("action ID reused with different payload")
	ErrCapacity  = errors.New("ledger capacity reached; provision a new ledger explicitly")
	ErrLocked    = errors.New("ledger lock exists; exclusive ownership required")
	ErrCorrupt   = errors.New("ledger corruption")
	ErrUncertain = errors.New("ledger write outcome uncertain; close and reopen before retry")
	ErrClosed    = errors.New("ledger closed")
)

type actionKey struct {
	Scope string
	ID    string
}

type record struct {
	Kind        string
	Key         actionKey
	Amount      int64  `json:",omitempty"`
	Total       int64  `json:",omitempty"`
	Fingerprint string `json:",omitempty"`
	EffectID    string `json:",omitempty"`
}

type entry struct {
	Record record
	Done   bool
}

// Effect.ID is the downstream consumer's durable deduplication key.
type Effect struct {
	ID    string
	Scope string
	Total int64
}

type ledgerFile interface {
	io.ReadWriteSeeker
	Sync() error
	Truncate(int64) error
	Close() error
}

type Ledger struct {
	mu       sync.Mutex
	file     ledgerFile
	lock     *os.File
	lockPath string
	entries  map[actionKey]entry
	order    []actionKey
	models   map[string]Model
	size     int64
	failed   error
	closed   bool
}

// Open owns path until Close. A pre-existing lock is never removed automatically.
func Open(path string) (_ *Ledger, err error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	path = filepath.Join(parent, filepath.Base(absolute))
	if info, statErr := os.Lstat(path); statErr == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("ledger must be a regular file, not a symlink")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, statErr
	}
	lockPath := path + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil, ErrLocked
	}
	if err != nil {
		return nil, err
	}
	l := &Ledger{
		lock:     lock,
		lockPath: lockPath,
		entries:  make(map[actionKey]entry),
		models:   make(map[string]Model),
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, l.Close())
		}
	}()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	l.file = file
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("%w: exceeds %d bytes", ErrCapacity, maxBytes)
	}
	fullSize := info.Size() / frameSize * frameSize
	for l.size < fullSize {
		var frame [frameSize]byte
		if _, err = io.ReadFull(file, frame[:]); err != nil {
			return nil, err
		}
		var rec record
		rec, err = decodeFrame(frame[:])
		if err != nil {
			return nil, fmt.Errorf("offset %d: %w", l.size, err)
		}
		if err = l.replay(rec); err != nil {
			return nil, fmt.Errorf("offset %d: %w", l.size, err)
		}
		l.size += frameSize
	}
	// Only an incomplete fixed-size final frame is recoverable. Full invalid
	// frames, including the last one, always fail closed.
	if info.Size() != fullSize {
		if err = file.Truncate(fullSize); err != nil {
			return nil, err
		}
	}
	// Sync also adopts complete frames left by an unsuccessful prior writer.
	if err = file.Sync(); err != nil {
		return nil, err
	}
	if _, err = file.Seek(fullSize, io.SeekStart); err != nil {
		return nil, err
	}
	return l, nil
}

func validKey(key actionKey) bool {
	return len(key.Scope) > 0 && len(key.Scope) <= 64 &&
		len(key.ID) > 0 && len(key.ID) <= 64 &&
		utf8.ValidString(key.Scope) && utf8.ValidString(key.ID)
}

func digest(value any) string {
	// Only fixed structs containing strings/integers are passed here.
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func actionRecord(key actionKey, msg Add, model Model) record {
	return record{
		Kind:        "action",
		Key:         key,
		Amount:      msg.Amount,
		Total:       model.Count,
		Fingerprint: digest(msg),
		EffectID:    digest(key),
	}
}

func encodeFrame(rec record) ([]byte, error) {
	data, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	if len(data) > checksumAt-12 {
		return nil, errors.New("record exceeds frame capacity")
	}
	frame := make([]byte, frameSize)
	copy(frame, magic)
	binary.BigEndian.PutUint32(frame[8:12], uint32(len(data)))
	copy(frame[12:], data)
	sum := sha256.Sum256(frame[:checksumAt])
	copy(frame[checksumAt:], sum[:])
	return frame, nil
}

func decodeFrame(frame []byte) (record, error) {
	var rec record
	sum := sha256.Sum256(frame[:checksumAt])
	if string(frame[:8]) != magic || !bytes.Equal(sum[:], frame[checksumAt:]) {
		return rec, ErrCorrupt
	}
	length := int(binary.BigEndian.Uint32(frame[8:12]))
	if length < 1 || length > checksumAt-12 {
		return rec, ErrCorrupt
	}
	if err := json.Unmarshal(frame[12:12+length], &rec); err != nil {
		return rec, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	canonical, err := encodeFrame(rec)
	if err != nil || !bytes.Equal(canonical, frame) {
		return rec, ErrCorrupt
	}
	return rec, nil
}

func (l *Ledger) replay(rec record) error {
	if !validKey(rec.Key) {
		return ErrCorrupt
	}
	switch rec.Kind {
	case "action":
		if _, exists := l.entries[rec.Key]; exists || len(l.entries) >= maxEntries {
			return ErrCorrupt
		}
		model, _, err := application.Update(
			context.Background(), l.models[rec.Key.Scope], Add{Amount: rec.Amount},
		)
		if err != nil || rec != actionRecord(rec.Key, Add{Amount: rec.Amount}, model) {
			return ErrCorrupt
		}
		l.commitAction(rec, model)
	case "done":
		item, exists := l.entries[rec.Key]
		if !exists || item.Done || rec != (record{Kind: "done", Key: rec.Key}) {
			return ErrCorrupt
		}
		item.Done = true
		l.entries[rec.Key] = item
	default:
		return ErrCorrupt
	}
	return nil
}

func (l *Ledger) ready() error {
	if l.closed {
		return ErrClosed
	}
	return l.failed
}

func (l *Ledger) append(rec record) error {
	if l.size+frameSize > maxBytes {
		return ErrCapacity
	}
	frame, err := encodeFrame(rec)
	if err != nil {
		return err
	}
	n, err := l.file.Write(frame)
	if err == nil && n != len(frame) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = l.file.Sync()
	}
	if err != nil {
		l.failed = errors.Join(ErrUncertain, err)
		return l.failed
	}
	l.size += frameSize
	return nil
}

func (l *Ledger) commitAction(rec record, model Model) {
	l.entries[rec.Key] = entry{Record: rec}
	l.order = append(l.order, rec.Key)
	l.models[rec.Key.Scope] = model
}

// Apply serializes concurrent duplicates. Its bool reports an existing action;
// the returned model is that action's original result, not the latest snapshot.
func (l *Ledger) Apply(
	ctx context.Context, scope, id string, msg Add,
) (Model, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.ready(); err != nil {
		return Model{}, false, err
	}
	if err := ctx.Err(); err != nil {
		return Model{}, false, err
	}
	key := actionKey{Scope: scope, ID: id}
	if !validKey(key) {
		return Model{}, false, errors.New("scope and action ID must be 1..64 UTF-8 bytes")
	}
	if item, exists := l.entries[key]; exists {
		if item.Record.Fingerprint != digest(msg) {
			return Model{}, false, ErrConflict
		}
		return Model{Count: item.Record.Total}, true, nil
	}
	if len(l.entries) >= maxEntries {
		return Model{}, false, ErrCapacity
	}
	model, _, err := application.Update(ctx, l.models[scope], msg)
	if err != nil {
		return Model{}, false, err
	}
	rec := actionRecord(key, msg, model)
	if err := l.append(rec); err != nil {
		return Model{}, false, err
	}
	l.commitAction(rec, model)
	return model, false, nil
}

func (l *Ledger) Snapshot(scope string) (Model, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.models[scope], l.ready()
}

func (l *Ledger) Pending() ([]Effect, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.ready(); err != nil {
		return nil, err
	}
	effects := make([]Effect, 0)
	for _, key := range l.order {
		item := l.entries[key]
		if !item.Done {
			effects = append(effects, Effect{
				ID:    item.Record.EffectID,
				Scope: key.Scope,
				Total: item.Record.Total,
			})
		}
	}
	return effects, nil
}

func (l *Ledger) complete(effectID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.ready(); err != nil {
		return err
	}
	for _, key := range l.order {
		item := l.entries[key]
		if item.Record.EffectID != effectID {
			continue
		}
		if item.Done {
			return nil
		}
		if err := l.append(record{Kind: "done", Key: key}); err != nil {
			return err
		}
		item.Done = true
		l.entries[key] = item
		return nil
	}
	return errors.New("unknown effect ID")
}

func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			// Keep ownership marker when closing the data file fails.
			return errors.Join(err, l.lock.Close())
		}
	}
	if err := l.lock.Close(); err != nil {
		return err
	}
	return os.Remove(l.lockPath)
}

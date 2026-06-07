package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func LoadTasks(store *TaskStore) error {
	filename := store.filename
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			store.recalcNextID()
			return nil
		}
		return fmt.Errorf("read tasks file: %w", err)
	}

	if len(data) == 0 {
		store.recalcNextID()
		return nil
	}

	var loaded struct {
		Tasks  []Task `json:"tasks"`
		NextID int    `json:"next_id"`
	}
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parse tasks json: %w", err)
	}

	store.Tasks = loaded.Tasks
	if loaded.NextID > 0 {
		store.NextID = loaded.NextID
	} else {
		store.recalcNextID()
	}

	return nil
}

func SaveTasks(store *TaskStore) error {
	filename := store.filename
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	data := struct {
		Tasks  []Task `json:"tasks"`
		NextID int    `json:"next_id"`
	}{
		Tasks:  store.Tasks,
		NextID: store.NextID,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}

	tmpFile := filename + ".tmp"
	if err := os.WriteFile(tmpFile, jsonData, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, filename); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("atomic rename: %w", err)
	}

	return nil
}

func WithWriteLock(filename string, fn func() error) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	lockFile := filename + ".lock"
	lock, err := acquireLock(lockFile)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer releaseLock(lock)

	return fn()
}

func WithReadLock(filename string, fn func() error) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	lockFile := filename + ".lock"
	lock, err := acquireLock(lockFile)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer releaseLock(lock)

	return fn()
}

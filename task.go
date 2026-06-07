package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Priority string

const (
	P0 Priority = "P0"
	P1 Priority = "P1"
	P2 Priority = "P2"
	P3 Priority = "P3"
)

var priorityOrder = map[Priority]int{
	P0: 0,
	P1: 1,
	P2: 2,
	P3: 3,
}

func ParsePriority(s string) (Priority, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch Priority(s) {
	case P0, P1, P2, P3:
		return Priority(s), nil
	default:
		return "", fmt.Errorf("invalid priority: %s (use P0, P1, P2, P3)", s)
	}
}

type Task struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Priority    Priority  `json:"priority"`
	Category    string    `json:"category"`
	DueDate     string    `json:"due_date"`
	Note        string    `json:"note"`
	Done        bool      `json:"done"`
	DoneAt      time.Time `json:"done_at"`
	Archived    bool      `json:"archived"`
	Deleted     bool      `json:"deleted"`
	DeletedAt   time.Time `json:"deleted_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TaskStore struct {
	Tasks    []Task `json:"tasks"`
	NextID   int    `json:"next_id"`
	config   *Config
	filename string
}

func NewTaskStore(cfg *Config) *TaskStore {
	store := &TaskStore{
		Tasks:    []Task{},
		NextID:   1,
		config:   cfg,
		filename: cfg.StoragePath,
	}
	return store
}

func (s *TaskStore) maxID() int {
	max := 0
	for _, t := range s.Tasks {
		if t.ID > max {
			max = t.ID
		}
	}
	return max
}

func (s *TaskStore) recalcNextID() {
	s.NextID = s.maxID() + 1
}

func (s *TaskStore) findDuplicate(title string) (*Task, bool) {
	now := time.Now()
	dedupDur := time.Duration(s.config.DedupMinutes) * time.Minute

	for i := range s.Tasks {
		t := &s.Tasks[i]
		if t.Deleted || t.Archived {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(t.Title), strings.TrimSpace(title)) {
			if now.Sub(t.CreatedAt) <= dedupDur {
				return t, true
			}
		}
	}
	return nil, false
}

func (s *TaskStore) AddTask(title string, priority Priority, category string, dueDate string, note string) (*Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("title cannot be empty")
	}

	if dup, found := s.findDuplicate(title); found {
		fmt.Printf("任务 \"%s\" 已在 %d 分钟内添加过(ID: %d)，跳过重复添加\n", title, s.config.DedupMinutes, dup.ID)
		return dup, nil
	}

	now := time.Now()
	task := Task{
		ID:        s.NextID,
		Title:     title,
		Priority:  priority,
		Category:  category,
		DueDate:   dueDate,
		Note:      note,
		Done:      false,
		Archived:  false,
		Deleted:   false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.Tasks = append(s.Tasks, task)
	s.NextID++
	return &s.Tasks[len(s.Tasks)-1], nil
}

func (s *TaskStore) GetTask(id int) (*Task, error) {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id && !s.Tasks[i].Deleted {
			return &s.Tasks[i], nil
		}
	}
	return nil, fmt.Errorf("task %d not found", id)
}

func (s *TaskStore) UpdateTask(id int, title, priority, category, dueDate, note *string) (*Task, error) {
	task, err := s.GetTask(id)
	if err != nil {
		return nil, err
	}

	if title != nil {
		task.Title = *title
	}
	if priority != nil {
		task.Priority = Priority(*priority)
	}
	if category != nil {
		task.Category = *category
	}
	if dueDate != nil {
		task.DueDate = *dueDate
	}
	if note != nil {
		task.Note = *note
	}
	task.UpdatedAt = time.Now()

	return task, nil
}

func (s *TaskStore) MarkDone(id int) (*Task, error) {
	task, err := s.GetTask(id)
	if err != nil {
		return nil, err
	}
	task.Done = true
	task.DoneAt = time.Now()
	task.UpdatedAt = time.Now()
	return task, nil
}

func (s *TaskStore) MarkUndone(id int) (*Task, error) {
	task, err := s.GetTask(id)
	if err != nil {
		return nil, err
	}
	task.Done = false
	task.DoneAt = time.Time{}
	task.UpdatedAt = time.Now()
	return task, nil
}

func (s *TaskStore) DeleteTask(id int) error {
	task, err := s.GetTask(id)
	if err != nil {
		return err
	}
	task.Deleted = true
	task.DeletedAt = time.Now()
	task.UpdatedAt = time.Now()
	return nil
}

func (s *TaskStore) ListTasks(includeDone bool, onlyDone bool, showArchived bool, categories []string, sortBy string) []Task {
	var result []Task

	for i := range s.Tasks {
		t := s.Tasks[i]

		if t.Deleted {
			continue
		}

		if showArchived {
			if !t.Archived {
				continue
			}
		} else {
			if t.Archived {
				continue
			}
		}

		if !showArchived {
			if onlyDone {
				if !t.Done {
					continue
				}
			} else if !includeDone {
				if t.Done {
					continue
				}
			}
		}

		if len(categories) > 0 {
			matched := false
			for _, cat := range categories {
				if strings.EqualFold(strings.TrimSpace(cat), strings.TrimSpace(t.Category)) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		result = append(result, t)
	}

	sort.Slice(result, func(i, j int) bool {
		return compareTasks(result[i], result[j], sortBy)
	})

	return result
}

func compareTasks(a, b Task, sortBy string) bool {
	switch sortBy {
	case "deadline":
		return compareByDeadline(a, b)
	case "created":
		return a.CreatedAt.After(b.CreatedAt)
	case "priority":
		fallthrough
	default:
		return compareByPriority(a, b)
	}
}

func compareByPriority(a, b Task) bool {
	pa := priorityOrder[a.Priority]
	pb := priorityOrder[b.Priority]
	if pa != pb {
		return pa < pb
	}
	return compareByDeadline(a, b)
}

func compareByDeadline(a, b Task) bool {
	if a.DueDate == "" && b.DueDate == "" {
		return a.CreatedAt.After(b.CreatedAt)
	}
	if a.DueDate == "" {
		return false
	}
	if b.DueDate == "" {
		return true
	}
	return a.DueDate < b.DueDate
}

func (s *TaskStore) PurgeOldDeletes(days int) int {
	cutoff := time.Now().AddDate(0, 0, -days)
	count := 0
	remaining := []Task{}
	for _, t := range s.Tasks {
		if t.Deleted && t.DeletedAt.Before(cutoff) {
			count++
		} else {
			remaining = append(remaining, t)
		}
	}
	s.Tasks = remaining
	return count
}

func (s *TaskStore) GetCategories() []string {
	seen := map[string]bool{}
	var cats []string
	for _, t := range s.Tasks {
		if t.Deleted {
			continue
		}
		cat := strings.TrimSpace(t.Category)
		if cat == "" {
			continue
		}
		if !seen[cat] {
			seen[cat] = true
			cats = append(cats, cat)
		}
	}
	sort.Strings(cats)
	return cats
}

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type CategoryStat struct {
	Category   string
	DoneCount  int
	TotalCount int
	TimeSpent  time.Duration
}

type WeeklyReport struct {
	WeekStart  time.Time
	WeekEnd    time.Time
	DoneCount  int
	TotalCount int
	ByCategory []CategoryStat
	DoneTasks  []Task
	OpenTasks  []Task
}

func startOfWeek(t time.Time) time.Time {
	year, month, day := t.Date()
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(year, month, day-weekday+1, 0, 0, 0, 0, t.Location())
}

func endOfWeek(t time.Time) time.Time {
	start := startOfWeek(t)
	return start.AddDate(0, 0, 7).Add(-time.Nanosecond)
}

func GenerateWeeklyReport(store *TaskStore) *WeeklyReport {
	now := time.Now()
	weekStart := startOfWeek(now)
	weekEnd := endOfWeek(now)

	report := &WeeklyReport{
		WeekStart: weekStart,
		WeekEnd:   weekEnd,
	}

	catMap := map[string]*CategoryStat{}

	for _, t := range store.Tasks {
		if t.Deleted || t.Archived {
			continue
		}

		inWeek := t.CreatedAt.After(weekStart) || t.CreatedAt.Equal(weekStart)
		if !inWeek {
			continue
		}

		cat := t.Category
		if cat == "" {
			cat = "未分类"
		}

		stat, ok := catMap[cat]
		if !ok {
			stat = &CategoryStat{Category: cat}
			catMap[cat] = stat
		}
		stat.TotalCount++
		report.TotalCount++

		if t.Done {
			stat.DoneCount++
			report.DoneCount++
			report.DoneTasks = append(report.DoneTasks, t)
			if !t.DoneAt.IsZero() {
				stat.TimeSpent += t.DoneAt.Sub(t.CreatedAt)
			}
		} else {
			report.OpenTasks = append(report.OpenTasks, t)
		}
	}

	for _, stat := range catMap {
		report.ByCategory = append(report.ByCategory, *stat)
	}

	sort.Slice(report.ByCategory, func(i, j int) bool {
		return report.ByCategory[i].DoneCount > report.ByCategory[j].DoneCount
	})

	sort.Slice(report.DoneTasks, func(i, j int) bool {
		return report.DoneTasks[i].DoneAt.After(report.DoneTasks[j].DoneAt)
	})

	sort.Slice(report.OpenTasks, func(i, j int) bool {
		return compareByPriority(report.OpenTasks[i], report.OpenTasks[j])
	})

	return report
}

func (r *WeeklyReport) Format() string {
	var b strings.Builder

	fmt.Fprintf(&b, "📅 本周周报 (%s ~ %s)\n",
		r.WeekStart.Format("2006-01-02"),
		r.WeekEnd.Format("2006-01-02"))
	fmt.Fprintf(&b, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Fprintf(&b, "✅ 已完成: %d  🔄 进行中: %d  📊 总数: %d\n",
		r.DoneCount, r.TotalCount-r.DoneCount, r.TotalCount)
	if r.TotalCount > 0 {
		fmt.Fprintf(&b, "📈 完成率: %.1f%%\n", float64(r.DoneCount)/float64(r.TotalCount)*100)
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "📂 按分类统计:")
	fmt.Fprintln(&b, "──────────────────────────────────")
	for _, s := range r.ByCategory {
		pct := 0.0
		if s.TotalCount > 0 {
			pct = float64(s.DoneCount) / float64(s.TotalCount) * 100
		}
		hours := s.TimeSpent.Hours()
		fmt.Fprintf(&b, "  %-12s %2d/%2d (%.0f%%)  耗时: %.1fh\n",
			s.Category, s.DoneCount, s.TotalCount, pct, hours)
	}
	fmt.Fprintln(&b)

	if len(r.DoneTasks) > 0 {
		fmt.Fprintln(&b, "✅ 本周已完成:")
		fmt.Fprintln(&b, "──────────────────────────────────")
		for _, t := range r.DoneTasks {
			fmt.Fprintf(&b, "  [%s] %s (ID: %d)\n", t.Priority, t.Title, t.ID)
		}
		fmt.Fprintln(&b)
	}

	if len(r.OpenTasks) > 0 {
		fmt.Fprintln(&b, "🔄 未完成任务:")
		fmt.Fprintln(&b, "──────────────────────────────────")
		for _, t := range r.OpenTasks {
			fmt.Fprintf(&b, "  [%s] %s (ID: %d)\n", t.Priority, t.Title, t.ID)
		}
	}

	return b.String()
}

func AutoArchive(store *TaskStore, days int) int {
	cutoff := time.Now().AddDate(0, 0, -days)
	count := 0
	for i := range store.Tasks {
		t := &store.Tasks[i]
		if t.Deleted || t.Archived {
			continue
		}
		if t.Done && !t.DoneAt.IsZero() && t.DoneAt.Before(cutoff) {
			t.Archived = true
			t.UpdatedAt = time.Now()
			count++
		}
	}
	return count
}

func UnarchiveTask(store *TaskStore, id int) (*Task, error) {
	for i := range store.Tasks {
		if store.Tasks[i].ID == id && !store.Tasks[i].Deleted && store.Tasks[i].Archived {
			store.Tasks[i].Archived = false
			store.Tasks[i].UpdatedAt = time.Now()
			return &store.Tasks[i], nil
		}
	}
	return nil, fmt.Errorf("archived task %d not found", id)
}

func ListArchivedTasks(store *TaskStore, categories []string) []Task {
	var result []Task
	for _, t := range store.Tasks {
		if t.Deleted || !t.Archived {
			continue
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
		return result[i].DoneAt.After(result[j].DoneAt)
	})
	return result
}

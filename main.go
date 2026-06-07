package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	cfg    *Config
	cfgErr error
)

func loadStore() (*TaskStore, error) {
	s := NewTaskStore(cfg)
	if err := WithReadLock(cfg.StoragePath, func() error {
		return LoadTasks(s)
	}); err != nil {
		return nil, err
	}
	return s, nil
}

func saveStore(s *TaskStore) error {
	return WithWriteLock(cfg.StoragePath, func() error {
		AutoArchive(s, cfg.ArchiveDays)
		s.PurgeOldDeletes(30)
		return SaveTasks(s)
	})
}

func priorityColor(p Priority) func(a ...interface{}) string {
	switch p {
	case P0:
		return color.New(color.FgRed, color.Bold).SprintFunc()
	case P1:
		return color.New(color.FgYellow, color.Bold).SprintFunc()
	case P2:
		return color.New(color.FgGreen).SprintFunc()
	default:
		return color.New(color.FgWhite).SprintFunc()
	}
}

func formatTaskLine(t Task) string {
	var status string
	if t.Done {
		status = color.GreenString("✅")
	} else {
		status = "  "
	}

	pcolor := priorityColor(t.Priority)
	priStr := pcolor(fmt.Sprintf("[%s]", t.Priority))

	title := t.Title
	if t.Done {
		title = color.New(color.FgHiBlack, color.CrossedOut).Sprint(t.Title)
	}

	deadline := ""
	if t.DueDate != "" {
		deadline = color.MagentaString(" 📅%s", t.DueDate)
	}

	cat := ""
	if t.Category != "" {
		cat = color.CyanString(" #%s", t.Category)
	}

	return fmt.Sprintf("%s %-5d %s %s%s%s", status, t.ID, priStr, title, cat, deadline)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", r)
			os.Exit(1)
		}
	}()

	cfg, cfgErr = LoadConfig()
	if cfg == nil {
		cfg = &defaultConfig
		cfg.StoragePath = defaultTasksFile()
	}
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", cfgErr)
	}

	color.NoColor = !isTerminal(os.Stdout)

	var rootCmd = &cobra.Command{
		Use:   "gocli-todo",
		Short: "跨平台终端待办事项管理器",
		Long:  `一个功能丰富的终端待办事项管理器，支持优先级、分类标签、周报等功能。`,
		SilenceErrors: true,
		SilenceUsage:  false,
	}

	rootCmd.AddCommand(newAddCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newDoneCmd())
	rootCmd.AddCommand(newUndoneCmd())
	rootCmd.AddCommand(newEditCmd())
	rootCmd.AddCommand(newDeleteCmd())
	rootCmd.AddCommand(newWeekCmd())
	rootCmd.AddCommand(newArchiveCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newCompletionCmd())
	rootCmd.AddCommand(newInstallCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func newAddCmd() *cobra.Command {
	var priority string
	var category string
	var dueDate string
	var note string

	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "添加新任务",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.Join(args, " ")

			pri, err := ParsePriority(priority)
			if err != nil {
				return err
			}

			s, err := loadStore()
			if err != nil {
				return err
			}

			task, err := s.AddTask(title, pri, category, dueDate, note)
			if err != nil {
				return err
			}

			if err := saveStore(s); err != nil {
				return err
			}

			fmt.Printf("✅ 已添加任务 #%d: %s\n", task.ID, task.Title)
			return nil
		},
	}

	cmd.Flags().StringVarP(&priority, "priority", "p", "P2", "优先级: P0, P1, P2, P3")
	cmd.Flags().StringVarP(&category, "category", "c", "", "分类标签")
	cmd.Flags().StringVarP(&dueDate, "due", "d", "", "截止时间 (YYYY-MM-DD)")
	cmd.Flags().StringVarP(&note, "note", "n", "", "备注")

	return cmd
}

func newListCmd() *cobra.Command {
	var showDone bool
	var showAll bool
	var sortBy string
	var categories []string
	var showArchived bool

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "列出任务",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadStore()
			if err != nil {
				return err
			}

			onlyDone := showDone && !showAll
			includeDone := showAll

			tasks := s.ListTasks(includeDone, onlyDone, showArchived, categories, sortBy)

			if len(tasks) == 0 {
				fmt.Println("暂无任务")
				return nil
			}

			label := "进行中"
			if onlyDone {
				label = "已完成"
			} else if includeDone {
				label = "全部"
			}
			if showArchived {
				label = "归档"
			}

			fmt.Printf("📋 %s任务 (%d 条)\n", label, len(tasks))
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			for _, t := range tasks {
				fmt.Println(formatTaskLine(t))
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&showDone, "done", "d", false, "显示已完成任务")
	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "显示所有任务")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "priority", "排序方式: priority, deadline, created")
	cmd.Flags().StringSliceVarP(&categories, "category", "c", nil, "分类标签过滤 (可多选)")
	cmd.Flags().BoolVarP(&showArchived, "archived", "A", false, "显示归档任务")

	return cmd
}

func newDoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "done [task-id]",
		Short: "标记任务为已完成",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid task id: %s", args[0])
			}

			s, err := loadStore()
			if err != nil {
				return err
			}

			task, err := s.MarkDone(id)
			if err != nil {
				return err
			}

			if err := saveStore(s); err != nil {
				return err
			}

			fmt.Printf("✅ 任务 #%d 已完成: %s\n", task.ID, task.Title)
			return nil
		},
	}
	return cmd
}

func newUndoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "undone [task-id]",
		Short: "取消任务的完成状态",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid task id: %s", args[0])
			}

			s, err := loadStore()
			if err != nil {
				return err
			}

			task, err := s.MarkUndone(id)
			if err != nil {
				return err
			}

			if err := saveStore(s); err != nil {
				return err
			}

			fmt.Printf("🔄 任务 #%d 已恢复: %s\n", task.ID, task.Title)
			return nil
		},
	}
	return cmd
}

func newEditCmd() *cobra.Command {
	var title string
	var priority string
	var category string
	var dueDate string
	var note string

	cmd := &cobra.Command{
		Use:   "edit [task-id]",
		Short: "编辑任务",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid task id: %s", args[0])
			}

			s, err := loadStore()
			if err != nil {
				return err
			}

			var titlePtr *string
			var priPtr *string
			var catPtr *string
			var duePtr *string
			var notePtr *string

			if cmd.Flags().Changed("title") {
				titlePtr = &title
			}
			if cmd.Flags().Changed("priority") {
				pri, err := ParsePriority(priority)
				if err != nil {
					return err
				}
				priStr := string(pri)
				priPtr = &priStr
			}
			if cmd.Flags().Changed("category") {
				catPtr = &category
			}
			if cmd.Flags().Changed("due") {
				duePtr = &dueDate
			}
			if cmd.Flags().Changed("note") {
				notePtr = &note
			}

			task, err := s.UpdateTask(id, titlePtr, priPtr, catPtr, duePtr, notePtr)
			if err != nil {
				return err
			}

			if err := saveStore(s); err != nil {
				return err
			}

			fmt.Printf("✏️  任务 #%d 已更新: %s\n", task.ID, task.Title)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "任务标题")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "优先级")
	cmd.Flags().StringVarP(&category, "category", "c", "", "分类标签")
	cmd.Flags().StringVarP(&dueDate, "due", "d", "", "截止时间")
	cmd.Flags().StringVarP(&note, "note", "n", "", "备注")

	return cmd
}

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [task-id]",
		Short: "删除任务 (软删除, 30天后永久删除)",
		Aliases: []string{"rm"},
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid task id: %s", args[0])
			}

			s, err := loadStore()
			if err != nil {
				return err
			}

			task, err := s.GetTask(id)
			if err != nil {
				return err
			}

			if err := s.DeleteTask(id); err != nil {
				return err
			}

			if err := saveStore(s); err != nil {
				return err
			}

			fmt.Printf("🗑️  任务 #%d 已删除: %s\n", task.ID, task.Title)
			return nil
		},
	}
	return cmd
}

func newWeekCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "week",
		Short: "生成本周周报",
		Aliases: []string{"report"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadStore()
			if err != nil {
				return err
			}

			report := GenerateWeeklyReport(s)
			fmt.Println(report.Format())
			return nil
		},
	}
	return cmd
}

func newArchiveCmd() *cobra.Command {
	var categories []string
	var unarchive bool

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "查看或管理归档任务",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadStore()
			if err != nil {
				return err
			}

			if unarchive {
				if len(args) == 0 {
					return fmt.Errorf("请指定要恢复的任务 ID")
				}
				id, err := strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("invalid task id: %s", args[0])
				}
				task, err := UnarchiveTask(s, id)
				if err != nil {
					return err
				}
				if err := saveStore(s); err != nil {
					return err
				}
				fmt.Printf("📤 任务 #%d 已从归档恢复: %s\n", task.ID, task.Title)
				return nil
			}

			tasks := ListArchivedTasks(s, categories)
			if len(tasks) == 0 {
				fmt.Println("暂无归档任务")
				return nil
			}

			fmt.Printf("📦 归档任务 (%d 条)\n", len(tasks))
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			for _, t := range tasks {
				doneAt := t.DoneAt.Format("2006-01-02")
				fmt.Printf("  #%-5d [%s] %s #%s (完成于 %s)\n",
					t.ID, t.Priority, t.Title, t.Category, doneAt)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&categories, "category", "c", nil, "分类标签过滤")
	cmd.Flags().BoolVarP(&unarchive, "unarchive", "u", false, "从归档恢复任务")

	return cmd
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "查看或修改配置",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("📁 配置文件: %s\n", defaultConfigFile())
			fmt.Printf("📁 数据文件: %s\n", cfg.StoragePath)
			fmt.Printf("⏰ 归档天数: %d 天\n", cfg.ArchiveDays)
			fmt.Printf("⏱️  去重窗口: %d 分钟\n", cfg.DedupMinutes)
			fmt.Printf("🎨 主题色 P0: %s\n", cfg.Theme.P0)
			fmt.Printf("🎨 主题色 P1: %s\n", cfg.Theme.P1)
			fmt.Printf("🎨 主题色 P2: %s\n", cfg.Theme.P2)
			fmt.Printf("🎨 主题色 P3: %s\n", cfg.Theme.P3)
			return nil
		},
	}

	cmd.AddCommand(newConfigSetCmd())
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	var storagePath string
	var archiveDays int
	var dedupMinutes int
	var p0, p1, p2, p3 string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "设置配置项",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("storage-path") {
				cfg.StoragePath = storagePath
			}
			if cmd.Flags().Changed("archive-days") {
				cfg.ArchiveDays = archiveDays
			}
			if cmd.Flags().Changed("dedup-minutes") {
				cfg.DedupMinutes = dedupMinutes
			}
			if cmd.Flags().Changed("p0") {
				cfg.Theme.P0 = p0
			}
			if cmd.Flags().Changed("p1") {
				cfg.Theme.P1 = p1
			}
			if cmd.Flags().Changed("p2") {
				cfg.Theme.P2 = p2
			}
			if cmd.Flags().Changed("p3") {
				cfg.Theme.P3 = p3
			}

			if err := SaveConfig(cfg); err != nil {
				return err
			}

			fmt.Println("✅ 配置已保存")
			return nil
		},
	}

	cmd.Flags().StringVar(&storagePath, "storage-path", "", "数据文件路径")
	cmd.Flags().IntVar(&archiveDays, "archive-days", 30, "归档天数")
	cmd.Flags().IntVar(&dedupMinutes, "dedup-minutes", 5, "去重窗口(分钟)")
	cmd.Flags().StringVar(&p0, "p0", "", "P0 颜色")
	cmd.Flags().StringVar(&p1, "p1", "", "P1 颜色")
	cmd.Flags().StringVar(&p2, "p2", "", "P2 颜色")
	cmd.Flags().StringVar(&p3, "p3", "", "P3 颜色")

	return cmd
}

func newCompletionCmd() *cobra.Command {
	var shell string

	cmd := &cobra.Command{
		Use:   "completion",
		Short: "生成 shell 补全脚本",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch shell {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			default:
				return fmt.Errorf("unsupported shell: %s (use bash, zsh, fish)", shell)
			}
		},
	}

	cmd.Flags().StringVarP(&shell, "shell", "s", "bash", "shell 类型: bash, zsh, fish")
	return cmd
}

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "安装 shell 补全到配置文件",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			completionDir := filepath.Join(defaultDataDir(), "completions")
			if err := os.MkdirAll(completionDir, 0755); err != nil {
				return err
			}

			bashFile := filepath.Join(completionDir, "gocli-todo.bash")
			zshFile := filepath.Join(completionDir, "_gocli-todo")
			fishFile := filepath.Join(completionDir, "gocli-todo.fish")

			root := cmd.Root()

			if f, err := os.Create(bashFile); err == nil {
				root.GenBashCompletion(f)
				f.Close()
			}

			if f, err := os.Create(zshFile); err == nil {
				root.GenZshCompletion(f)
				f.Close()
			}

			if f, err := os.Create(fishFile); err == nil {
				root.GenFishCompletion(f, true)
				f.Close()
			}

			zshrc := filepath.Join(home, ".zshrc")
			bashrc := filepath.Join(home, ".bashrc")

			zshLine := fmt.Sprintf("\n# gocli-todo completion\nif [ -f \"%s\" ]; then\n  fpath=(%s $fpath)\n  autoload -U compinit && compinit\nfi\n", zshFile, completionDir)
			bashLine := fmt.Sprintf("\n# gocli-todo completion\nif [ -f \"%s\" ]; then\n  source \"%s\"\nfi\n", bashFile, bashFile)

			installed := false

			if _, err := os.Stat(zshrc); err == nil {
				data, err := os.ReadFile(zshrc)
				if err == nil && !strings.Contains(string(data), "gocli-todo completion") {
					if err := appendToFile(zshrc, zshLine); err == nil {
						fmt.Printf("✅ 已安装 zsh 补全到 %s\n", zshrc)
						installed = true
					}
				} else if err == nil {
					fmt.Printf("ℹ️  zsh 补全已安装\n")
					installed = true
				}
			}

			if _, err := os.Stat(bashrc); err == nil {
				data, err := os.ReadFile(bashrc)
				if err == nil && !strings.Contains(string(data), "gocli-todo completion") {
					if err := appendToFile(bashrc, bashLine); err == nil {
						fmt.Printf("✅ 已安装 bash 补全到 %s\n", bashrc)
						installed = true
					}
				} else if err == nil {
					fmt.Printf("ℹ️  bash 补全已安装\n")
					installed = true
				}
			}

			if !installed {
				fmt.Println("ℹ️  未检测到 .zshrc 或 .bashrc，请手动安装补全:")
				fmt.Printf("  zsh:  source %s\n", zshFile)
				fmt.Printf("  bash: source %s\n", bashFile)
			} else {
				fmt.Println("\n请重新打开终端或 source 对应的配置文件以启用补全。")
			}

			return nil
		},
	}

	return cmd
}

func appendToFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	iconParentDir = "⬆️ "
	iconDirectory = "📁"
	iconFile      = "📄"
	iconCurrent   = "> "

	titlePrefix   = "📁 "
	titleColor    = "62"
	selectedColor = "170"
	helpColor     = "241"
)

var (
	itemStyle         = lipgloss.NewStyle().PaddingLeft(2)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color(selectedColor))
	titleStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color(titleColor)).Bold(true)
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color(helpColor)).Padding(1, 0, 0, 2)
)

type dirEntry struct {
	entry fs.DirEntry
}

func (d dirEntry) FilterValue() string {
	return d.entry.Name()
}

func (d dirEntry) icon() string {
	if !d.entry.IsDir() {
		return iconFile
	}
	if d.entry.Name() == ".." {
		return iconParentDir
	}
	return iconDirectory
}

type parentDirEntry struct{}

func (p *parentDirEntry) Name() string               { return ".." }
func (p *parentDirEntry) IsDir() bool                { return true }
func (p *parentDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (p *parentDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

type itemDelegate struct{}

func (d itemDelegate) Height() int  { return 1 }
func (d itemDelegate) Spacing() int { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(dirEntry)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s %s", item.icon(), item.entry.Name())

	if index == m.Index() {
		fmt.Fprint(w, selectedItemStyle.Render(iconCurrent+str))
		return
	}
	fmt.Fprint(w, itemStyle.Render(str))
}

type model struct {
	initialPath string
	currentPath string
	list        list.Model
	err         error
	copyPath    bool
	copyFormat  string
}

type errMsg struct {
	err error
}

type dirChangeMsg struct {
	path    string
	entries []fs.DirEntry
}

func newModel() (model, error) {
	currentPath, err := os.Getwd()
	if err != nil {
		return model{}, err
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return model{}, err
	}

	items := createListItems(entries)
	l := createList(items, currentPath)

	return model{
		initialPath: currentPath,
		currentPath: currentPath,
		list:        l,
	}, nil
}

func createList(items []list.Item, path string) list.Model {
	l := list.New(items, itemDelegate{}, 0, 0)
	l.Title = titlePrefix + path
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	return l
}

func createListItems(entries []fs.DirEntry) []list.Item {
	dirs, files := partitionEntries(entries)

	sortByName(dirs)
	sortByName(files)

	sortedEntries := make([]fs.DirEntry, 0, len(entries)+1)
	sortedEntries = append(sortedEntries, &parentDirEntry{})
	sortedEntries = append(sortedEntries, dirs...)
	sortedEntries = append(sortedEntries, files...)

	return convertToListItems(sortedEntries)
}

func partitionEntries(entries []fs.DirEntry) (dirs, files []fs.DirEntry) {
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}
	return dirs, files
}

func sortByName(entries []fs.DirEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
}

func convertToListItems(entries []fs.DirEntry) []list.Item {
	items := make([]list.Item, len(entries))
	for i, entry := range entries {
		items[i] = dirEntry{entry: entry}
	}
	return items
}

func (m model) Init() tea.Cmd {
	return tea.SetWindowTitle("cd-plus - Interactive Directory Navigator")
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowResize(msg), nil
	case dirChangeMsg:
		return m.handleDirChange(msg), nil
	case errMsg:
		m.err = msg.err
		return m, nil
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) handleWindowResize(msg tea.WindowSizeMsg) model {
	m.list.SetWidth(msg.Width)
	m.list.SetHeight(msg.Height - 2)
	return m
}

func (m model) handleDirChange(msg dirChangeMsg) model {
	m.currentPath = msg.path
	items := createListItems(msg.entries)
	m.list.SetItems(items)
	m.list.Title = titlePrefix + msg.path
	m.list.ResetSelected()
	m.list.ResetFilter()
	return m
}

func (m model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		m.copyPath = false
		return m, tea.Quit
	case "f":
		m.copyPath = true
		m.copyFormat = "f"
		return m, tea.Quit
	case "r":
		m.copyPath = true
		m.copyFormat = "r"
		return m, tea.Quit
	case "a":
		m.copyPath = true
		m.copyFormat = "a"
		return m, tea.Quit
	case "d":
		m.copyPath = true
		m.copyFormat = "d"
		return m, tea.Quit
	case "p":
		m.copyPath = true
		m.copyFormat = "p"
		return m, tea.Quit
	case "enter", " ", "l", "right":
		return m.handleEnterDirectory()
	case "backspace", "h", "left":
		return m, m.navigateToParent()
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) handleEnterDirectory() (tea.Model, tea.Cmd) {
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		return m, nil
	}

	entry := selectedItem.(dirEntry).entry
	if !entry.IsDir() {
		return m, nil
	}

	return m, m.navigateTo(entry.Name())
}

func (m model) navigateTo(dirname string) tea.Cmd {
	return func() tea.Msg {
		newPath := m.resolveNewPath(dirname)
		entries, err := os.ReadDir(newPath)
		if err != nil {
			return errMsg{err}
		}

		return dirChangeMsg{
			path:    newPath,
			entries: entries,
		}
	}
}

func (m model) navigateToParent() tea.Cmd {
	return m.navigateTo("..")
}

func (m model) resolveNewPath(dirname string) string {
	if dirname == ".." {
		return filepath.Dir(m.currentPath)
	}
	return filepath.Join(m.currentPath, dirname)
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.\n", m.err)
	}

	helpText := helpStyle.Render(
		"Copy: f (file) • r (relative) • a (absolute) • d (directory path) • p (`$HOME` format)",
	)

	return m.list.View() + "\n" + helpText
}

func (m model) getSelectedPath() string {
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		return m.currentPath
	}

	entry := selectedItem.(dirEntry).entry
	if entry.Name() == ".." {
		return m.currentPath
	}

	return filepath.Join(m.currentPath, entry.Name())
}

func (m model) getFormattedPath() (string, error) {
	selectedPath := m.getSelectedPath()

	switch m.copyFormat {
	case "f":
		return filepath.Base(selectedPath), nil

	case "r":
		relPath, err := filepath.Rel(m.initialPath, selectedPath)
		if err != nil {
			return "", fmt.Errorf("failed to get relative path: %w", err)
		}
		return relPath, nil

	case "a":
		absPath, err := filepath.Abs(selectedPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}
		return absPath, nil

	case "d":
		absPath, err := filepath.Abs(selectedPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			return filepath.Dir(absPath), nil
		}
		return absPath, nil

	case "p":
		absPath, err := filepath.Abs(selectedPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}
		dirPath := absPath
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			dirPath = filepath.Dir(absPath)
		}

		homeDir, err := os.UserHomeDir()
		if err == nil {
			relPath, err := filepath.Rel(homeDir, dirPath)
			if err == nil && !filepath.IsAbs(relPath) && len(relPath) > 0 && relPath[0] != '.' {
				return filepath.Join("$HOME", relPath), nil
			}
		}
		return dirPath, nil

	default:
		return selectedPath, nil
	}
}

func run() error {
	m, err := newModel()
	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running program: %w", err)
	}

	if m, ok := finalModel.(model); ok && m.copyPath {
		pathToCopy, err := m.getFormattedPath()
		if err != nil {
			return fmt.Errorf("failed to format path: %w", err)
		}

		if err := clipboard.WriteAll(pathToCopy); err != nil {
			return fmt.Errorf("failed to write to clipboard: %w", err)
		}
		fmt.Println(pathToCopy)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

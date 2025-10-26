package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	itemStyle         = lipgloss.NewStyle().PaddingLeft(2)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color("170"))
)

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(dirEntry)
	if !ok {
		return
	}

	icon := "📄"
	if item.entry.IsDir() {
		if item.entry.Name() == ".." {
			icon = "⬆️ "
		} else {
			icon = "📁"
		}
	}

	str := fmt.Sprintf("%s %s", icon, item.entry.Name())

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + s[0])
		}
	}

	fmt.Fprint(w, fn(str))
}

type dirEntry struct {
	entry fs.DirEntry
}

func (d dirEntry) FilterValue() string {
	return d.entry.Name()
}

type model struct {
	currentPath string
	list        list.Model
	err         error
}

func initialModel() model {
	currentPath, err := os.Getwd()
	if err != nil {
		return model{err: err}
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return model{err: err, currentPath: currentPath}
	}

	items := makeListItems(entries)

	delegate := itemDelegate{}
	l := list.New(items, delegate, 0, 0)
	l.Title = "📁 " + currentPath
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true)

	return model{
		currentPath: currentPath,
		list:        l,
	}
}

func makeListItems(entries []fs.DirEntry) []list.Item {
	sortedEntries := make([]fs.DirEntry, 0, len(entries)+1)
	sortedEntries = append(sortedEntries, &parentDirEntry{})

	var dirs, files []fs.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name() < dirs[j].Name()
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	sortedEntries = append(sortedEntries, dirs...)
	sortedEntries = append(sortedEntries, files...)

	items := make([]list.Item, len(sortedEntries))
	for i, entry := range sortedEntries {
		items[i] = dirEntry{entry: entry}
	}
	return items
}

type parentDirEntry struct{}

func (p *parentDirEntry) Name() string               { return ".." }
func (p *parentDirEntry) IsDir() bool                { return true }
func (p *parentDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (p *parentDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func (m model) Init() tea.Cmd {
	return tea.SetWindowTitle("cd-plus - Interactive Directory Navigator")
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 2)
		return m, nil

	case dirChangeMsg:
		m.currentPath = msg.path
		items := makeListItems(msg.entries)
		m.list.SetItems(items)
		m.list.Title = "📁 " + msg.path
		m.list.ResetSelected()
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

		switch msg.String() {
		case "enter", "l", "right":
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				entry := selectedItem.(dirEntry).entry
				if entry.IsDir() {
					return m, m.changeDirectory(entry.Name())
				}
			}
			return m, nil

		case "h", "left":
			return m, m.changeDirectory("..")
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) changeDirectory(dirname string) tea.Cmd {
	return func() tea.Msg {
		var newPath string
		if dirname == ".." {
			newPath = filepath.Dir(m.currentPath)
		} else {
			newPath = filepath.Join(m.currentPath, dirname)
		}

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

type errMsg struct {
	err error
}

type dirChangeMsg struct {
	path    string
	entries []fs.DirEntry
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.\n", m.err)
	}

	return m.list.View()
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}

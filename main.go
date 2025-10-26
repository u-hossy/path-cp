package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

type mode string

const (
	ModeSelect mode = "select"
	ModeSearch mode = "search"
)

type model struct {
	mode        mode
	cursor      int
	currentPath string
	entries     []fs.DirEntry
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

	return model{
		mode:        ModeSelect,
		currentPath: currentPath,
		entries:     sortedEntries,
		cursor:      0,
	}
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
	switch m.mode {
	case ModeSelect:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "/":
				m.mode = ModeSearch
				return m, nil
			case "ctrl+c", "q":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.entries)-1 {
					m.cursor++
				}
			case "enter", "l", "right":
				if m.cursor < len(m.entries) {
					entry := m.entries[m.cursor]
					if entry.IsDir() {
						var newPath string
						if entry.Name() == ".." {
							newPath = filepath.Dir(m.currentPath)
						} else {
							newPath = filepath.Join(m.currentPath, entry.Name())
						}

						entries, err := os.ReadDir(newPath)
						if err != nil {
							m.err = err
							return m, nil
						}

						sortedEntries := make([]fs.DirEntry, 0, len(entries)+1)
						sortedEntries = append(sortedEntries, &parentDirEntry{})

						var dirs, files []fs.DirEntry
						for _, e := range entries {
							if e.IsDir() {
								dirs = append(dirs, e)
							} else {
								files = append(files, e)
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

						m.currentPath = newPath
						m.entries = sortedEntries
						m.cursor = 0
						m.err = nil
					}
				}
			case "h", "left":
				newPath := filepath.Dir(m.currentPath)
				entries, err := os.ReadDir(newPath)
				if err != nil {
					m.err = err
					return m, nil
				}

				sortedEntries := make([]fs.DirEntry, 0, len(entries)+1)
				sortedEntries = append(sortedEntries, &parentDirEntry{})

				var dirs, files []fs.DirEntry
				for _, e := range entries {
					if e.IsDir() {
						dirs = append(dirs, e)
					} else {
						files = append(files, e)
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

				m.currentPath = newPath
				m.entries = sortedEntries
				m.cursor = 0
				m.err = nil
			}
		}
	case ModeSearch:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.mode = ModeSelect
				return m, nil
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.\n", m.err)
	}

	s := fmt.Sprintf("📁 Current Directory: %s\n\n", m.currentPath)

	for i, entry := range m.entries {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		icon := "📄"
		if entry.IsDir() {
			if entry.Name() == ".." {
				icon = "⬆️ "
			} else {
				icon = "📁"
			}
		}

		s += fmt.Sprintf("%s %s %s\n", cursor, icon, entry.Name())
	}

	s += "\n"
	s += "Navigation: ↑/k (up) ↓/j (down) Enter/l/→ (enter dir) h/← (parent dir)\n"
	s += "Press q to quit.\n"

	return s
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}

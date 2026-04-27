package filesync

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"cms/internal/globals"
	_ "cms/internal/globals"
	"cms/internal/render"

	"github.com/sgtdi/fswatcher"
)

// What a pain in the ass to test.
func TestFirstSync(t *testing.T) {
	tests := []struct {
		name        string
		wantErr     bool
		files       []string
		wantedFiles []string
	}{
		{"2 md", false, []string{"lol.md", filepath.Join("adir", "funny.md")}, []string{"lol", filepath.Join("adir", "funny")}},
		{"2 md 1 png", false, []string{"haha.md", filepath.Join("dir", "lol.png"), filepath.Join("dir", "ımkındadümbaşş.md")}, []string{"haha", filepath.Join("dir", "ımkındadümbaşş")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := mockDB(t)
			tempDir := t.TempDir()
			mdDir := filepath.Join(tempDir, "mdDir")
			pageDir := filepath.Join(tempDir, "assets", "pages")
			err := os.MkdirAll(mdDir, 0o777)
			if err != nil {
				t.Fatal("Couldn't create mdDir!")
			}
			err = os.MkdirAll(pageDir, 0o777)
			if err != nil {
				t.Fatal("Couldn't create pageDir!")
			}
			globals.AssetsPath = filepath.Join(tempDir, "assets")
			os.MkdirAll(filepath.Join(globals.AssetsPath, "templates"), 0o777)
			os.Create(filepath.Join(globals.AssetsPath, "templates", "base.tmpl"))
			os.MkdirAll(filepath.Join(globals.AssetsPath, "homePage"), 0o777)
			os.Create(filepath.Join(globals.AssetsPath, "homePage", "base.tmpl"))
			os.MkdirAll(filepath.Join(globals.AssetsPath, "atom"), 0o777)
			os.Create(filepath.Join(globals.AssetsPath, "atom", "atom.tmpl"))
			for _, file := range test.files {
				os.MkdirAll(filepath.Dir(filepath.Join(mdDir, file)), 0o777)
				os.Create(filepath.Join(mdDir, file))
			}
			rdrconf := render.RenderConfig{"Test", "", "", "", "", 20, 20}
			err = FirstSync(mdDir, db, &rdrconf)

			if (err != nil && test.wantErr == false) || (err == nil && test.wantErr == true) {
				errText := ""
				if err != nil {
					errText = err.Error()
				}
				t.Fatalf("Error wantErr mismatch. wantErr %v. Error msg: %v", test.wantErr, errText)
			}
			foundMd := []string{}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			res, err := db.QueryContext(ctx, "SELECT filename FROM checksums")
			if err != nil {
				t.Fatal("Query failed: " + err.Error())
			}
			for res.Next() {
				filename := ""
				res.Scan(&filename)
				foundMd = append(foundMd, filename)
			}
			if len(foundMd) != len(test.wantedFiles) {
				t.Fatalf("Lengths don't match between foundMd and wantedFiles. foundMd: %v wantedFiles: %v", foundMd, test.wantedFiles)
			}
			pagesGenerated := []string{}
			filepath.WalkDir(filepath.Join(globals.AssetsPath, "pages"), func(path string, d fs.DirEntry, err error) error {
				if stat, _ := os.Stat(path); stat.IsDir() {
					return nil
				}
				pagesGenerated = append(pagesGenerated, path)
				return nil
			})
			if len(test.wantedFiles) != len(pagesGenerated) {
				t.Fatalf("wantedFiles and pagesGenerated have different lengths. wantedFiles: %v pagesGenerated: %v", test.wantedFiles, pagesGenerated)
			}
			for _, file := range test.wantedFiles {
				if !slices.Contains(foundMd, filepath.Join(mdDir, file)+".md") {
					t.Fatalf("%v not found in foundMd. foundMd: %v", filepath.Join(mdDir, file)+".md", foundMd)
				}
				if !slices.Contains(pagesGenerated, filepath.Join(globals.AssetsPath, "pages", file)+".html.br") {
					t.Fatalf("%v not found in pagesGenerated. pagesGenerated: %v", filepath.Join(globals.AssetsPath, "pages", file)+".html.br", pagesGenerated)
				}
			}
		})
	}
}

// How am i gonna handle Sync()? I don't know. It's even more of a pain in the ass.
// Ok shit below is %99 AI is generated because i can't be bothered to do all of this. Seems to work.
// Anyways i at least decoupled Sync() from hardcoded fs.Watcher so it can be tested better.
// MockWatcher implements the exact fswatcher.Watcher interface for testing.
type MockWatcher struct {
	mu          sync.Mutex
	eventsChan  chan fswatcher.WatchEvent
	droppedChan chan fswatcher.WatchEvent
	paths       []string
	running     bool
}

func NewMockWatcher() *MockWatcher {
	return &MockWatcher{
		// Buffered channels so test events don't block
		eventsChan:  make(chan fswatcher.WatchEvent, 10),
		droppedChan: make(chan fswatcher.WatchEvent, 10),
	}
}

func (m *MockWatcher) Watch(ctx context.Context) error {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	// Block until the context is canceled, mimicking real watcher behavior
	<-ctx.Done()

	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	return nil
}

func (m *MockWatcher) AddPath(path string, options ...fswatcher.PathOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paths = append(m.paths, path)
	return nil
}

func (m *MockWatcher) DropPath(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, p := range m.paths {
		if p == path {
			m.paths = append(m.paths[:i], m.paths[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockWatcher) Events() <-chan fswatcher.WatchEvent {
	return m.eventsChan
}

func (m *MockWatcher) Dropped() <-chan fswatcher.WatchEvent {
	return m.droppedChan
}

func (m *MockWatcher) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *MockWatcher) Stats() fswatcher.WatcherStats {
	return fswatcher.WatcherStats{} // Return empty/dummy stats
}

func (m *MockWatcher) Paths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to prevent race conditions
	pathsCopy := make([]string, len(m.paths))
	copy(pathsCopy, m.paths)
	return pathsCopy
}

func (m *MockWatcher) Close() {
	// Close channels to signal readers to exit
	close(m.eventsChan)
	close(m.droppedChan)
}

func TestProcessSync_Events(t *testing.T) {
	tests := []struct {
		name            string
		initialFiles    []string // Files to create BEFORE the watcher starts
		delayedFiles    []string // Files to create AFTER the watcher starts (simulating real-time creation)
		preCreateHTML   []string
		simulatedEvents []fswatcher.WatchEvent
		expectHTMLFiles []string
		expectDeleted   []string
	}{
		{
			name:         "Create new markdown file",
			delayedFiles: []string{"new_post.md"}, // Moved to delayed!
			simulatedEvents: []fswatcher.WatchEvent{
				{Path: "new_post.md", Types: []fswatcher.EventType{fswatcher.EventCreate}},
			},
			expectHTMLFiles: []string{"new_post.html.br"},
		},
		{
			name:          "Delete markdown file",
			preCreateHTML: []string{"old_post.html.br"},
			simulatedEvents: []fswatcher.WatchEvent{
				{Path: "old_post.md", Types: []fswatcher.EventType{fswatcher.EventRemove}},
			},
			expectDeleted: []string{"old_post.html.br"},
		},
		{
			name:          "Rename markdown file",
			delayedFiles:  []string{"new_name.md"}, // The "new" file is created during the test
			preCreateHTML: []string{"old_name.html.br"},
			simulatedEvents: []fswatcher.WatchEvent{
				{Path: "old_name.md", Types: []fswatcher.EventType{fswatcher.EventRename}},
				{Path: "new_name.md", Types: []fswatcher.EventType{fswatcher.EventCreate}},
			},
			expectHTMLFiles: []string{"new_name.html.br"},
			expectDeleted:   []string{"old_name.html.br"},
		},
		{
			name: "Create folder with contents",
			// Moved to delayedFiles so the initial WalkDir doesn't catch it
			delayedFiles: []string{filepath.Join("my_folder", "nested_post.md")},
			simulatedEvents: []fswatcher.WatchEvent{
				{Path: "my_folder", Types: []fswatcher.EventType{fswatcher.EventCreate}},
			},
			expectHTMLFiles: []string{filepath.Join("my_folder", "nested_post.html.br")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := mockDB(t)
			tempDir := t.TempDir()
			mdDir := filepath.Join(tempDir, "mdDir")
			globals.AssetsPath = filepath.Join(tempDir, "assets")
			os.MkdirAll(mdDir, 0o777)
			os.MkdirAll(filepath.Join(globals.AssetsPath, "pages"), 0o777)
			os.MkdirAll(filepath.Join(globals.AssetsPath, "templates"), 0o777)
			os.Create(filepath.Join(globals.AssetsPath, "templates", "base.tmpl"))
			os.MkdirAll(filepath.Join(globals.AssetsPath, "homePage"), 0o777)
			os.Create(filepath.Join(globals.AssetsPath, "homePage", "base.tmpl"))
			os.MkdirAll(filepath.Join(globals.AssetsPath, "atom"), 0o777)
			os.Create(filepath.Join(globals.AssetsPath, "atom", "atom.tmpl"))
			// 1. Setup initial files (Before Watcher Starts)
			for _, file := range tc.initialFiles {
				fullPath := filepath.Join(mdDir, file)
				os.MkdirAll(filepath.Dir(fullPath), 0o777)
				os.WriteFile(fullPath, []byte("# Test"), 0o644)
			}

			for _, file := range tc.preCreateHTML {
				fullPath := filepath.Join(globals.AssetsPath, "pages", file)
				os.MkdirAll(filepath.Dir(fullPath), 0o777)
				os.WriteFile(fullPath, []byte("<html></html>"), 0o644)
			}

			mockWatcher := NewMockWatcher()
			logger := slog.Default()
			rdrconf := &render.RenderConfig{SiteName: "Test"}
			ctx, cancel := context.WithCancel(context.Background())

			// 2. Start the processor
			go processSync(ctx, mockWatcher, db, mdDir, logger, rdrconf)

			// Yield briefly to ensure the initial WalkDir inside processSync finishes
			time.Sleep(10 * time.Millisecond)

			// 3. Setup delayed files (Simulating OS file creations)
			for _, file := range tc.delayedFiles {
				fullPath := filepath.Join(mdDir, file)
				os.MkdirAll(filepath.Dir(fullPath), 0o777)
				os.WriteFile(fullPath, []byte("# Test"), 0o644)
			}

			// 4. Fire events
			for _, ev := range tc.simulatedEvents {
				ev.Path = filepath.Join(mdDir, ev.Path)
				mockWatcher.eventsChan <- ev
				time.Sleep(5 * time.Millisecond)
			}

			// Yield to let the goroutine finish processing
			time.Sleep(20 * time.Millisecond)

			// 5. Assertions
			for _, expectedHTML := range tc.expectHTMLFiles {
				target := filepath.Join(globals.AssetsPath, "pages", expectedHTML)
				if _, err := os.Stat(target); os.IsNotExist(err) {
					t.Errorf("Expected HTML file %s was not generated", expectedHTML)
				}
			}

			for _, expectedDeleted := range tc.expectDeleted {
				target := filepath.Join(globals.AssetsPath, "pages", expectedDeleted)
				if _, err := os.Stat(target); err == nil {
					t.Errorf("Expected HTML file %s to be deleted, but it still exists", expectedDeleted)
				}
			}

			cancel()
		})
	}
}

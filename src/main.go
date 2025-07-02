// 2025-07-02
package main

import (
	"bytes"
	"fmt"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	uploadURL    = "http://localhost:4711/upload"
	uploadFolder = "./upload"
)

func startServer() {
	os.MkdirAll(uploadFolder, 0755)

	http.HandleFunc("/upload/", func(w http.ResponseWriter, r *http.Request) {
		relPath := strings.TrimPrefix(r.URL.Path, "/upload/")
		localPath := filepath.Join(uploadFolder, filepath.FromSlash(relPath))

		os.MkdirAll(filepath.Dir(localPath), 0755)

		outFile, err := os.Create(localPath)
		if err != nil {
			http.Error(w, "Create failed: "+err.Error(), 500)
			return
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, r.Body)
		if err != nil {
			http.Error(w, "Write failed: "+err.Error(), 500)
			return
		}

		// Änderungszeit übernehmen
		modTimeStr := r.Header.Get("X-File-ModTime")
		if modTimeStr != "" {
			if modTime, err := time.Parse(time.RFC3339, modTimeStr); err == nil {
				os.Chtimes(localPath, modTime, modTime)
			}
		}

		w.WriteHeader(http.StatusCreated)
	})

	go http.ListenAndServe(":8080", nil)
}

func uploadFile(fullPath string, baseDir string) error {
	relPath, err := filepath.Rel(baseDir, fullPath)
	if err != nil {
		return err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, file)
	if err != nil {
		return err
	}

	putURL := uploadURL + "/" + filepath.ToSlash(relPath)
	req, err := http.NewRequest("PUT", putURL, buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-File-ModTime", stat.ModTime().UTC().Format(time.RFC3339))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Upload fehlgeschlagen: %s", resp.Status)
	}

	return nil
}

func walkAndUpload(path string, te *walk.TextEdit) {
	info, err := os.Stat(path)
	if err != nil {
		te.SetText("Fehler beim Öffnen: " + err.Error())
		return
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Dir(path)
	} else {
		baseDir = filepath.Dir(path)
	}

	var allFiles []string
	if info.IsDir() {
		filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			allFiles = append(allFiles, p)
			return nil
		})
	} else {
		allFiles = []string{path}
	}

	for _, f := range allFiles {
		te.SetText("Sende: " + f)
		err := uploadFile(f, baseDir)
		if err != nil {
			te.SetText("Fehler: " + err.Error())
			return
		}
	}

	te.SetText("Fertig. " + fmt.Sprintf("%d Datei(en) übertragen.", len(allFiles)))
}

func main() {
	startServer()

	var mw *walk.MainWindow
	var te *walk.TextEdit

	MainWindow{
		AssignTo: &mw,
		Title:    "Lokaler Datei-Uploader & Server",
		MinSize:  Size{Width: 500, Height: 300},
		Layout:   VBox{},
		Children: []Widget{
			TextEdit{
				AssignTo: &te,
				ReadOnly: true,
				Text:     "Ziehe eine Datei oder ein Verzeichnis hierher.",
				VScroll:  true,
			},
		},
		OnDropFiles: func(paths []string) {
			if len(paths) == 0 {
				return
			}

			go walkAndUpload(paths[0], te)
		},
	}.Run()
}

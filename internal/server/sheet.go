package server

import (
	"path/filepath"
	"sort"

	"os"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

const EXTENSION = ".paisa"

type SheetFile struct {
	Name      string   `json:"name"`
	Content   string   `json:"content"`
	Versions  []string `json:"versions"`
	Operation string   `json:"operation"`
}

func GetSheets(deps Deps) SheetFilesResponse {
	db := deps.DB
	dir := deps.Config.SheetDir()
	paths, _ := doublestar.FilepathGlob(dir + "/**/*" + EXTENSION)

	files := []SheetFile{}
	for _, path := range paths {
		files = append(files, *readSheetFileWithVersions(dir, path))
	}

	postings := query.Init(db).All()
	postings = service.PopulateMarketPrice(db, postings)

	return SheetFilesResponse{Files: files, Postings: nonNilSlice(postings)}
}

func GetSheet(file SheetFile, store *config.Store) SheetFileResponse {
	dir := store.SheetDir()
	filePath, err := utils.BuildSubPath(dir, file.Name)
	if err != nil {
		return SheetFileResponse{Message: "Invalid file name"}
	}
	return SheetFileResponse{File: readSheetFile(dir, filePath)}
}

func DeleteSheetBackups(file SheetFile, store *config.Store) SheetFileResponse {
	dir := store.SheetDir()
	filePath, err := utils.BuildSubPath(dir, file.Name)
	if err != nil {
		return SheetFileResponse{Message: "Invalid file name"}
	}

	if !store.Get().Readonly {
		versions := utils.Glob(filePath, ".backup.*")
		for _, version := range versions {
			err := os.Remove(version)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	return SheetFileResponse{File: readSheetFileWithVersions(dir, filePath)}
}

func SaveSheetFile(deps Deps, file SheetFile) SheetSaveResponse {
	dir := deps.Config.SheetDir()

	filePath, err := utils.BuildSubPath(dir, file.Name)
	if err != nil {
		log.Warn(err)
		return SheetSaveResponse{Saved: false, Message: "Invalid file name"}
	}

	now := utils.Now()
	backupPath := filePath + ".backup." + now.Format("2006-01-02-15-04-05.000")

	err = os.MkdirAll(filepath.Dir(filePath), 0700)
	if err != nil {
		log.Warn(err)
		return SheetSaveResponse{Saved: false, Message: "Failed to create directory"}
	}

	fileStat, err := os.Stat(filePath)
	if err != nil && file.Operation != "overwrite" && file.Operation != "create" {
		log.Warn(err)
		return SheetSaveResponse{Saved: false, Message: "File does not exist"}
	}

	var perm os.FileMode = 0644
	if err == nil {
		if file.Operation == "create" {
			return SheetSaveResponse{Saved: false, Message: "File already exists"}
		}

		perm = fileStat.Mode().Perm()
		existingContent, err := os.ReadFile(filePath)
		if err != nil {
			log.Warn(err)
			return SheetSaveResponse{Saved: false, Message: "Failed to read file"}
		}

		err = os.WriteFile(backupPath, existingContent, perm)
		if err != nil {
			log.Warn(err)
			return SheetSaveResponse{Saved: false, Message: "Failed to create backup"}
		}
	}

	err = os.WriteFile(filePath, []byte(file.Content), perm)
	if err != nil {
		log.Warn(err)
		return SheetSaveResponse{Saved: false, Message: "Failed to write file"}
	}

	pruneOldBackups(filePath, now)

	return SheetSaveResponse{Saved: true, File: readSheetFileWithVersions(dir, filePath)}
}

func readSheetFile(dir string, path string) *SheetFile {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	name, err := filepath.Rel(dir, path)

	return &SheetFile{
		Name:     name,
		Content:  string(content),
		Versions: []string{},
	}
}

func readSheetFileWithVersions(dir string, path string) *SheetFile {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	versions := utils.Glob(path, ".backup.*")
	versionPaths := lo.Map(versions, func(path string, _ int) string {
		name, err := filepath.Rel(dir, path)
		if err != nil {
			log.Fatal(err)
		}

		return name
	})
	sort.Sort(sort.Reverse(sort.StringSlice(versionPaths)))

	name, err := filepath.Rel(dir, path)
	if err != nil {
		log.Fatal(err)
	}

	return &SheetFile{
		Name:     name,
		Content:  string(content),
		Versions: versionPaths,
	}
}

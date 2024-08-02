package main

import (
	"embed"
	_ "embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

//go:embed *.sql
var migration embed.FS

func main() {
	dest := "."
	if len(os.Args) > 1 {
		dest = os.Args[1]
		if err := os.MkdirAll(dest, os.ModePerm); err != nil {
			log.Fatalln("fail to create target directory:", err)
		}
	}
	if err := dist(migration, dest); err != nil {
		log.Fatalln("fail to copy to target directory:", err)
	}
}

func dist(efs embed.FS, destDir string) error {
	return fs.WalkDir(efs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(".", path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, os.ModePerm)
		}

		srcFile, err := efs.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, srcFile, 0666)
	})
}

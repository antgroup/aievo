package index

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/antgroup/aievo/rag"
)

func BaseDocuments(_ context.Context, args *rag.WorkflowContext) error {
	stat, err := os.Stat(args.BasePath)
	if err != nil {
		return errors.New(
			"failed to stat workflow files, err: " + err.Error())
	}
	if !stat.IsDir() {
		content, err := os.ReadFile(args.BasePath)
		if err != nil {
			return errors.New(
				"failed to load file, err: " + err.Error())
		}
		args.Documents = append(args.Documents, &rag.Document{
			Id:      id(filepath.Base(args.BasePath)),
			Title:   filepath.Base(args.BasePath),
			Content: string(content),
		})
		return nil
	}

	type dir struct {
		name  string
		path  string
		isDir bool
	}

	paths := make([]dir, 0, 10)
	paths = append(paths, dir{"", "", stat.IsDir()})

	for i := 0; i < len(paths); i++ {
		if paths[i].name == "." || paths[i].name == ".." {
			continue
		}
		if !paths[i].isDir {
			content, err := os.ReadFile(filepath.Join(
				args.BasePath, paths[i].path))
			if err != nil {
				return err
			}
			args.Documents = append(args.Documents, &rag.Document{
				Id:      id(paths[i].path),
				Title:   paths[i].path,
				Content: string(content),
			})
			continue
		}
		// 循环遍历目录
		entries, err := os.ReadDir(filepath.Join(args.BasePath,
			paths[i].path))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			paths = append(paths, dir{
				name: entry.Name(),
				path: filepath.Join(paths[i].path,
					entry.Name()),
				isDir: entry.IsDir(),
			})
		}
	}

	return nil
}

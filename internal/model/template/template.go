package template

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

//go:embed all:templates
var BuiltinTemplates embed.FS

type TemplateType string

const (
	Builtin TemplateType = "builtin"
	Custom  TemplateType = "custom"
)

type Template struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Content      string       `json:"content"`
	TemplateType TemplateType `json:"template_type"`
}

func All() []Template {
	return all(config.GetConfig().ImportTemplates, BuiltinTemplates)
}

func all(custom []config.ImportTemplate, builtins fs.FS) []Template {
	templates := []Template{}

	for _, t := range custom {
		template := Template{ID: buildID(t.Name, Custom), Name: t.Name, Content: t.Content, TemplateType: Custom}
		templates = append(templates, template)
	}

	dirEntries, err := fs.ReadDir(builtins, "templates")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return templates
		}
		log.Fatal(err)
	}

	for _, f := range dirEntries {
		name := f.Name()
		if f.IsDir() || path.Ext(name) != ".handlebars" {
			continue
		}

		content, err := fs.ReadFile(builtins, path.Join("templates", name))
		if err != nil {
			log.Fatal(err)
		}

		name = strings.TrimSuffix(name, path.Ext(name))
		template := Template{ID: buildID(name, Builtin), Name: name, Content: string(content), TemplateType: Builtin}
		templates = append(templates, template)
	}

	return templates
}

func Upsert(name string, content string) (Template, error) {
	template := Template{ID: buildID(name, Custom), Name: name, Content: content, TemplateType: Custom}

	if config.GetConfig().Readonly {
		return template, nil
	}

	if err := Delete(name); err != nil {
		return template, err
	}
	cfg := config.GetConfig()
	cfg.ImportTemplates = append(cfg.ImportTemplates, config.ImportTemplate{Name: name, Content: content})
	err := config.SaveConfigObject(cfg)
	if err != nil {
		return template, err
	}

	return template, nil
}

func Delete(name string) error {
	cfg := config.GetConfig()
	cfg.ImportTemplates = lo.Filter(cfg.ImportTemplates, func(t config.ImportTemplate, _ int) bool {
		return t.Name != name
	})

	return config.SaveConfigObject(cfg)
}

func buildID(name string, templateType TemplateType) string {
	return fmt.Sprintf("%s:%s", templateType, name)
}

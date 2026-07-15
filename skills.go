package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Skill declarativa: um arquivo markdown com frontmatter (name/description) + corpo
// (instruções que aumentam o system prompt base). Compartilhável com a extensão.
type Skill struct {
	Name        string
	Description string
	System      string
}

func parseSkill(txt string) Skill {
	var sk Skill
	if strings.HasPrefix(txt, "---") {
		partes := strings.SplitN(txt, "---", 3)
		if len(partes) == 3 {
			for _, ln := range strings.Split(partes[1], "\n") {
				ln = strings.TrimSpace(ln)
				if strings.HasPrefix(ln, "name:") {
					sk.Name = strings.TrimSpace(ln[len("name:"):])
				} else if strings.HasPrefix(ln, "description:") {
					sk.Description = strings.TrimSpace(ln[len("description:"):])
				}
			}
			sk.System = strings.TrimSpace(partes[2])
			return sk
		}
	}
	sk.System = strings.TrimSpace(txt)
	return sk
}

func carregarSkills(dir string) map[string]Skill {
	res := map[string]Skill{}
	if dir == "" {
		return res
	}
	arqs, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	for _, a := range arqs {
		b, err := os.ReadFile(a)
		if err != nil {
			continue
		}
		s := parseSkill(string(b))
		if s.Name == "" {
			s.Name = strings.TrimSuffix(filepath.Base(a), ".md")
		}
		res[s.Name] = s
	}
	return res
}

// carregarAgentes = skills embutidas (skillsDir) + agentes/skills do WORKSPACE
// (.vitruvio/agents e .vitruvio/skills). Workspace tem precedência em nomes iguais.
// As instruções resultantes SEMPRE passam pela constituição no servidor (anti-bypass).
func carregarAgentes(root, skillsDir string) map[string]Skill {
	res := carregarSkills(skillsDir)
	for _, d := range []string{
		filepath.Join(root, ".vitruvio", "agents"),
		filepath.Join(root, ".vitruvio", "skills"),
	} {
		for k, v := range carregarSkills(d) {
			res[k] = v
		}
	}
	return res
}

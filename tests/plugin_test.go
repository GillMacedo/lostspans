package tests

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(root(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// The version is the same everywhere it appears.
func TestVersions(t *testing.T) {
	var plugin struct{ Name, Version string }
	readJSON(t, "plugin/.claude-plugin/plugin.json", &plugin)
	if plugin.Name != "lostspans" {
		t.Errorf("plugin name %q", plugin.Name)
	}
	r := runTool(t, nil, "--version")
	if strings.TrimSpace(r.stdout) != "lostspans "+plugin.Version {
		t.Errorf("--version says %q, plugin.json says %s", r.stdout, plugin.Version)
	}
	changelog, _ := os.ReadFile(root("CHANGELOG.md"))
	if !strings.Contains(string(changelog), "## ["+plugin.Version+"]") {
		t.Errorf("CHANGELOG.md has no section for %s", plugin.Version)
	}
	readme, _ := os.ReadFile(root("README.md"))
	if !strings.Contains(string(readme), "@v"+plugin.Version) {
		t.Errorf("README.md doesn't pin v%s", plugin.Version)
	}
}

func TestMarketplace(t *testing.T) {
	var m struct {
		Name    string
		Plugins []struct{ Name, Source string }
	}
	readJSON(t, ".claude-plugin/marketplace.json", &m)
	if m.Name != "lostspans-marketplace" || len(m.Plugins) != 1 || m.Plugins[0].Name != "lostspans" || m.Plugins[0].Source != "./plugin" {
		t.Errorf("marketplace: %+v", m)
	}
}

func TestSkillFrontmatter(t *testing.T) {
	data, err := os.ReadFile(root("plugin/skills/checking-opentelemetry-env/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)^---\nname: (\S+)\ndescription: ([^\n]+)\n---\n`).FindStringSubmatch(string(data))
	if m == nil {
		t.Fatal("no frontmatter")
	}
	if m[1] != "checking-opentelemetry-env" {
		t.Errorf("name %q doesn't match the folder", m[1])
	}
	if len(m[2]) > 1024 {
		t.Errorf("description is %d characters, over 1024", len(m[2]))
	}
	// Every rule the README lists is named in the skill's ranges.
	readme, _ := os.ReadFile(root("README.md"))
	for _, id := range regexp.MustCompile("`(LS\\d{3})`").FindAllStringSubmatch(string(readme), -1) {
		n := id[1]
		group := n[:4]
		if !strings.Contains(string(data), group) {
			t.Errorf("the skill doesn't mention the %sxx rules", group)
		}
	}
}

// Every rule in the code is in the README's table, and the other way round.
func TestRulesDocumented(t *testing.T) {
	src, err := os.ReadFile(root("plugin/scripts/rules.go"))
	if err != nil {
		t.Fatal(err)
	}
	readme, _ := os.ReadFile(root("README.md"))
	inCode := map[string]bool{}
	for _, m := range regexp.MustCompile(`"(LS\d{3})"`).FindAllStringSubmatch(string(src), -1) {
		inCode[m[1]] = true
	}
	inTable := map[string]bool{}
	for _, m := range regexp.MustCompile("(?m)^\\| `(LS\\d{3})` \\|").FindAllStringSubmatch(string(readme), -1) {
		inTable[m[1]] = true
	}
	for id := range inCode {
		if !inTable[id] {
			t.Errorf("%s is in rules.go but not in the README table", id)
		}
	}
	for id := range inTable {
		if !inCode[id] {
			t.Errorf("%s is in the README table but not in rules.go", id)
		}
	}
	if len(inCode) < 25 {
		t.Errorf("found only %d rules", len(inCode))
	}
}

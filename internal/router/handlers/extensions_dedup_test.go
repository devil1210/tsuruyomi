package handlers

import (
	"context"
	"strings"
	"testing"

	"tsuruyomi/internal/database"
)

func TestGetExtensionNodes_Deduplication(t *testing.T) {
	ctx := context.Background()
	nodes := getExtensionNodes(ctx, "")

	seenNameLang := make(map[string]string) // key -> pkgName
	for _, node := range nodes {
		name, _ := node["name"].(string)
		lang, _ := node["lang"].(string)
		pkgName, _ := node["pkgName"].(string)

		key := strings.ToLower(strings.TrimSpace(name)) + "::" + strings.ToLower(strings.TrimSpace(lang))
		if prevPkg, exists := seenNameLang[key]; exists {
			t.Fatalf("Duplicate extension detected in getExtensionNodes: %q (lang: %s) appears in both %q and %q", name, lang, prevPkg, pkgName)
		}
		seenNameLang[key] = pkgName
	}
}

func TestKnownSourcesMapping(t *testing.T) {
	targetIDs := []struct {
		id          int64
		expectedSub string
		lang        string
	}{
		{4097111295486074350, "NineManga", "es"},
		{120391793502126753, "NineManga", "en"},
		{1326771034671033762, "Manhwas.net", "es"},
		{3752522006902890093, "LMTO Online", "es"},
		{7732736838916328250, "Lmto", "es"},
		{4531444389842992129, "Bato.to", "all"},
		{3903750533251938295, "Jobsibe", "es"},
		{8590845094935985528, "Zevep", "es"},
		{7541593615312027809, "DragonTranslation", "es"},
	}

	for _, tt := range targetIDs {
		info, ok := database.KnownSourcesByID[tt.id]
		if !ok {
			t.Errorf("Target source ID %d not found in database.KnownSourcesByID", tt.id)
			continue
		}

		if !strings.Contains(strings.ToLower(info.Name), strings.ToLower(tt.expectedSub)) {
			t.Errorf("Source %d name = %q; expected to contain %q", tt.id, info.Name, tt.expectedSub)
		}

		if info.Lang != tt.lang {
			t.Errorf("Source %d lang = %q; expected %q", tt.id, info.Lang, tt.lang)
		}

		if info.PkgName == "" || strings.HasPrefix(info.PkgName, "imported.source.") {
			t.Errorf("Source %d has invalid package name %q", tt.id, info.PkgName)
		}
	}
}

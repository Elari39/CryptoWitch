package vault

import "testing"

func TestBuildTreeUsesNormalizedFullPaths(t *testing.T) {
	tree := BuildTree([]PlainDocument{
		{
			ID:    "nested",
			Title: "Nested",
			Path:  `guide\advanced\nested.md`,
		},
		{
			ID:    "intro",
			Title: "Intro",
			Path:  "./guide/intro.md",
		},
	})

	if len(tree) != 1 {
		t.Fatalf("len(tree) = %d, want 1: %#v", len(tree), tree)
	}
	guide := tree[0]
	if guide.Kind != "folder" || guide.Title != "guide" || guide.Path != "guide" {
		t.Fatalf("guide node = %#v, want folder path guide", guide)
	}
	if len(guide.Children) != 2 {
		t.Fatalf("len(guide.Children) = %d, want 2: %#v", len(guide.Children), guide.Children)
	}

	advanced := guide.Children[0]
	if advanced.Kind != "folder" || advanced.Title != "advanced" || advanced.Path != "guide/advanced" {
		t.Fatalf("advanced node = %#v, want folder path guide/advanced", advanced)
	}
	if len(advanced.Children) != 1 || advanced.Children[0].Path != "guide/advanced/nested.md" {
		t.Fatalf("advanced children = %#v, want nested document with normalized full path", advanced.Children)
	}

	intro := guide.Children[1]
	if intro.Kind != "document" || intro.ID != "intro" || intro.Path != "guide/intro.md" {
		t.Fatalf("intro node = %#v, want document path guide/intro.md", intro)
	}
}

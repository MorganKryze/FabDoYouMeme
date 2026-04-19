package game

import "testing"

func TestPackRoleString(t *testing.T) {
	if string(PackRoleImage) != "image" {
		t.Fatalf("PackRoleImage = %q, want %q", PackRoleImage, "image")
	}
	if string(PackRoleText) != "text" {
		t.Fatalf("PackRoleText = %q, want %q", PackRoleText, "text")
	}
}

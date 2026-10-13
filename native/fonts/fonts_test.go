package fonts

import "testing"

func TestFacesCoverEveryWeightTheWindowDraws(t *testing.T) {
	faces, err := Faces()
	if err != nil {
		t.Fatal(err)
	}
	if len(faces) != len(weights) {
		t.Fatalf("got %d faces, want %d", len(faces), len(weights))
	}
	for i, f := range faces {
		if f.Font.Typeface != Sans || f.Font.Weight != weights[i].weight {
			t.Errorf("face %d: %+v", i, f.Font)
		}
	}
}

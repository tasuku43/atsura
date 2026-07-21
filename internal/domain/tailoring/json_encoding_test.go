package tailoring

import (
	"encoding/json"
	"testing"
)

func TestJSONValueMarshalPreservesOrderLexicalNumbersAndTypes(t *testing.T) {
	value := NewJSONObject([]JSONField{
		{Name: "z", Value: NewJSONNumber("1.2300e+04")},
		{Name: "a", Value: NewJSONArray([]JSONValue{NewJSONNull(), NewJSONBool(false), NewJSONString("line\n\\literal")})},
	})
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"z":1.2300e+04,"a":[null,false,"line\n\\literal"]}`
	if string(encoded) != want {
		t.Fatalf("encoded=%s want=%s", encoded, want)
	}
}

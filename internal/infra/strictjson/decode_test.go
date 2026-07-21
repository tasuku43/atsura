package strictjson

import "testing"

type fixture struct {
	Name string `json:"name"`
	Data struct {
		Value int `json:"value"`
	} `json:"data"`
}

func TestDecodeStrictObject(t *testing.T) {
	var value fixture
	if err := Decode([]byte(`{"name":"one","data":{"value":2}}`), &value, 8); err != nil || value.Data.Value != 2 {
		t.Fatalf("Decode() = %+v, %v", value, err)
	}
}

func TestDecodeRejectsDuplicateUnknownTrailingAndDepth(t *testing.T) {
	values := []string{
		`{"name":"one","name":"two","data":{"value":2}}`,
		`{"name":"one","unknown":true,"data":{"value":2}}`,
		`{"name":"one","data":{"value":2}} {}`,
		`{"name":"one","data":{"value":2}}`,
	}
	depths := []int{8, 8, 8, 1}
	for index, raw := range values {
		var value fixture
		if err := Decode([]byte(raw), &value, depths[index]); err == nil {
			t.Fatalf("Decode(%s) succeeded", raw)
		}
	}
}

package jsonvalue

import (
	"encoding/json"
	"testing"
)

func TestRoundTripBusinessJSON(t *testing.T) {
	for _, literal := range []string{`9007199254740993`, `123456789012345678901234567890`, `0.12345678901234567890123`, `1e-10000000`, `false`, `null`, `{"a":[9007199254740993,false,null],"properties":{"item_ids":{"default":9007199254740993}}}`} {
		var value any
		if err := Unmarshal([]byte(literal), &value); err != nil {
			t.Fatal(err)
		}
		actual, err := json.Marshal(value)
		if err != nil || string(actual) != literal {
			t.Fatalf("%s -> %s (%v)", literal, actual, err)
		}
	}
}

func TestTypedFieldsRawSnapshotsAndOrdinaryNumbers(t *testing.T) {
	type envelope struct {
		Revision int
		Value    any
		Nested   map[string]any
		Snapshot json.RawMessage
	}
	var out envelope
	if err := Unmarshal([]byte(`{"Revision":7,"Value":0.25,"Nested":{"large":9007199254740993,"ordinary":42},"Snapshot":{ "b":2, "a":9007199254740993 }}`), &out); err != nil {
		t.Fatal(err)
	}
	if out.Revision != 7 || out.Value != float64(0.25) || out.Nested["ordinary"] != float64(42) || out.Nested["large"] != json.Number("9007199254740993") {
		t.Fatalf("types changed: %#v", out)
	}
	if string(out.Snapshot) != `{ "b":2, "a":9007199254740993 }` {
		t.Fatal("raw snapshot changed")
	}
	for _, raw := range []string{`{} {}`, `[1,`, ``} {
		if err := Unmarshal([]byte(raw), &out); err == nil {
			t.Fatal("accepted invalid JSON", raw)
		}
	}
}

func TestEquivalentNumericSpellingsKeepFloatCompatibility(t *testing.T) {
	for _, raw := range []string{`1.00`, `1e3`, `10000000000000000000000`, `-0`, `0e99999999999999999999`, `0.1`, `0.95`, `-12.5`} {
		var value any
		if err := Unmarshal([]byte(raw), &value); err != nil {
			t.Fatal(err)
		}
		if _, ok := value.(float64); !ok {
			t.Fatalf("ordinary number changed type: %s %#v", raw, value)
		}
	}
}

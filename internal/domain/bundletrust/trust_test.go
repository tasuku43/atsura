package bundletrust

import "testing"

func TestStoreUsesSortedExactDigestReceipts(t *testing.T) {
	a := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	b := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	store, changed, err := EmptyStore().Add(b)
	if err != nil || !changed {
		t.Fatalf("Add(b) = changed %v, error %v", changed, err)
	}
	store, changed, err = store.Add(a)
	if err != nil || !changed || !store.Contains(a) || !store.Contains(b) || store.Receipts[0].BundleDigest != a {
		t.Fatalf("store = %+v, changed %v, error %v", store, changed, err)
	}
	unchanged, changed, err := store.Add(a)
	if err != nil || changed || len(unchanged.Receipts) != 2 {
		t.Fatalf("duplicate add = %+v, changed %v, error %v", unchanged, changed, err)
	}
}

func TestStoreRejectsInvalidOrUnsortedReceipts(t *testing.T) {
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, store := range []Store{
		{},
		{SchemaVersion: 2, Receipts: []Receipt{}},
		{SchemaVersion: 1, Receipts: []Receipt{{BundleDigest: "no"}}},
		{SchemaVersion: 1, Receipts: []Receipt{{BundleDigest: digest}, {BundleDigest: digest}}},
	} {
		if err := store.Validate(); err == nil {
			t.Fatalf("Validate(%+v) succeeded", store)
		}
	}
}

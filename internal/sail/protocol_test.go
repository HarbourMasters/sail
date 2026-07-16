package sail

import (
	"encoding/json"
	"testing"
)

// TestSubscriptionWireFormat pins the exact subscribe/unsubscribe JSON,
// guarding the protocol v2 field names (hookName/hookIdFilter) against a
// struct-tag change — the game rejects the old eventName/eventIdFilter.
// Examples from mm/2s2h/Network/Sail/PROTOCOL_v2_CHANGES.md.
func TestSubscriptionWireFormat(t *testing.T) {
	idFilter := 10

	cases := []struct {
		name string
		pkt  subscriptionPacket
		want string
	}{
		{
			name: "subscribe, unfiltered — hookIdFilter omitted entirely",
			pkt:  subscriptionPacket{ID: "7", Type: PacketTypeSubscribe, HookName: HookOnItemGive},
			want: `{"id":"7","type":"subscribe","hookName":"OnItemGive"}`,
		},
		{
			name: "subscribe, filtered",
			pkt:  subscriptionPacket{ID: "7", Type: PacketTypeSubscribe, HookName: HookOnItemGive, HookIDFilter: &idFilter},
			want: `{"id":"7","type":"subscribe","hookName":"OnItemGive","hookIdFilter":10}`,
		},
		{
			name: "unsubscribe, filtered",
			pkt:  subscriptionPacket{ID: "8", Type: PacketTypeUnsubscribe, HookName: HookOnItemGive, HookIDFilter: &idFilter},
			want: `{"id":"8","type":"unsubscribe","hookName":"OnItemGive","hookIdFilter":10}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.pkt)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("wire format drifted\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

// TestFilterID pins which field each hook is id-filtered by, mirroring the
// game's Sail.cpp: scene/item/actor init are filterable, the flag hooks are
// not.
func TestFilterID(t *testing.T) {
	cases := []struct {
		hook       Hook
		wantID     int
		wantFilter bool
	}{
		{Hook{Type: "OnSceneInit", SceneID: 12}, 12, true},
		{Hook{Type: "OnItemGive", ItemID: 4}, 4, true},
		{Hook{Type: "OnActorInit", ActorID: 80}, 80, true},
		{Hook{Type: "OnFlagSet", Flag: 5}, 0, false},
		{Hook{Type: "OnSceneFlagSet", SceneID: 3}, 0, false},
		{Hook{Type: "Unknown", SceneID: 9}, 0, false},
	}
	for _, tc := range cases {
		id, filterable := tc.hook.FilterID()
		if id != tc.wantID || filterable != tc.wantFilter {
			t.Errorf("%s.FilterID() = (%d, %v), want (%d, %v)", tc.hook.Type, id, filterable, tc.wantID, tc.wantFilter)
		}
	}
}

// TestKnownHookCatalog checks the compiled-in catalog covers every known hook
// and derives idFilter from FilterID (so the two can't disagree).
func TestKnownHookCatalog(t *testing.T) {
	catalog := KnownHookCatalog()
	if len(catalog) != len(KnownHookNames) {
		t.Fatalf("catalog has %d entries, want %d", len(catalog), len(KnownHookNames))
	}

	filterable := map[string]bool{}
	for _, h := range catalog {
		filterable[h.Name] = h.IDFilter
	}
	if !filterable["OnItemGive"] {
		t.Error("OnItemGive should be id-filterable in the catalog")
	}
	if filterable["OnFlagSet"] {
		t.Error("OnFlagSet should not be id-filterable in the catalog")
	}
}

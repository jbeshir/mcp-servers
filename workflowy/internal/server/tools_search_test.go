package server

import (
	"testing"

	"github.com/jbeshir/mcp-servers/workflowy/internal/client"
)

const queryFoo = "foo"

func ptr[T any](v T) *T { return &v }

func TestValidateSearchArgs(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		completedAfter  *int64
		completedBefore *int64
		want            bool
	}{
		{"all empty → invalid", "", nil, nil, false},
		{"query only → valid", queryFoo, nil, nil, true},
		{"after only → valid", "", ptr(int64(1000)), nil, true},
		{"before only → valid", "", nil, ptr(int64(2000)), true},
		{"both bounds → valid", "", ptr(int64(1000)), ptr(int64(2000)), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateSearchArgs(tc.query, tc.completedAfter, tc.completedBefore); got != tc.want {
				t.Errorf("validateSearchArgs(%q, %v, %v) = %v, want %v",
					tc.query, tc.completedAfter, tc.completedBefore, got, tc.want)
			}
		})
	}
}

func TestSearchNodes(t *testing.T) {
	ts100 := int64(100)
	ts200 := int64(200)
	ts300 := int64(300)

	// Build a small node set:
	//   nodeA: uncompleted, name "foo"
	//   nodeB: completed at t=200, name "foo"
	//   nodeC: completed at t=100, name "bar"
	//   nodeD: completed at t=300, name "foo"
	//   nodeE: uncompleted, name "bar"
	//   nodeF: nil CompletedAt (completed flag via Completed field), name "foo"
	nodeA := client.Node{ID: "a", Name: queryFoo}
	nodeB := client.Node{ID: "b", Name: queryFoo, CompletedAt: &ts200}
	nodeC := client.Node{ID: "c", Name: "bar", CompletedAt: &ts100}
	nodeD := client.Node{ID: "d", Name: queryFoo, CompletedAt: &ts300}
	nodeE := client.Node{ID: "e", Name: "bar"}
	nodeF := client.Node{ID: "f", Name: queryFoo, Completed: ptr(true)}

	nodes := []client.Node{nodeA, nodeB, nodeC, nodeD, nodeE, nodeF}
	index := buildIndex(nodes)

	type args struct {
		queryLower      string
		filterCompleted *bool
		completedAfter  *int64
		completedBefore *int64
		limit           int
	}

	tests := []struct {
		name    string
		args    args
		wantIDs []string
	}{
		{
			// THE TRAP: completed_after with no `completed` arg (filterCompleted=nil).
			// Default-exclude must be bypassed; completed nodes with CompletedAt >= 150 returned.
			name: "completed_after only (trap: default-exclude bypassed)",
			args: args{
				queryLower:      "",
				filterCompleted: nil,
				completedAfter:  ptr(int64(150)),
				limit:           200,
			},
			wantIDs: []string{"b", "d"},
		},
		{
			name: "completed_before only",
			args: args{
				queryLower:      "",
				filterCompleted: nil,
				completedBefore: ptr(int64(200)),
				limit:           200,
			},
			// nodes with CompletedAt <= 200: b(200), c(100)
			wantIDs: []string{"b", "c"},
		},
		{
			name: "both bounds with boundary inclusivity (upper bound exact match included)",
			args: args{
				queryLower:      "",
				filterCompleted: nil,
				completedAfter:  ptr(int64(100)),
				completedBefore: ptr(int64(200)),
				limit:           200,
			},
			// c(100) inclusive, b(200) inclusive, d(300) excluded
			wantIDs: []string{"b", "c"},
		},
		{
			name: "nil CompletedAt excluded when date bound set",
			args: args{
				queryLower:      "",
				filterCompleted: nil,
				completedAfter:  ptr(int64(0)),
				limit:           200,
			},
			// nodeF has Completed=true but nil CompletedAt → excluded by matchesDateRange
			wantIDs: []string{"b", "c", "d"},
		},
		{
			name: "query + date bound combine with AND",
			args: args{
				queryLower:      queryFoo,
				filterCompleted: nil,
				completedAfter:  ptr(int64(150)),
				limit:           200,
			},
			// must match "foo" AND CompletedAt >= 150: b(200,"foo"), d(300,"foo")
			wantIDs: []string{"b", "d"},
		},
		{
			// Regression: query-only, no date args, filterCompleted=nil → default-exclude applies.
			// Only uncompleted "foo" nodes (nodeA). nodeB, nodeD, nodeF are completed.
			name: "regression: query-only default-exclude still applies",
			args: args{
				queryLower: queryFoo,
				limit:      200,
			},
			wantIDs: []string{"a"},
		},
		{
			// Regression: completed=true returns completed matches.
			name: "regression: completed=true returns completed matches",
			args: args{
				queryLower:      queryFoo,
				filterCompleted: ptr(true),
				limit:           200,
			},
			// completed "foo": b, d (CompletedAt non-nil), f (Completed=true)
			wantIDs: []string{"b", "d", "f"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := searchNodes(nodes, index,
				tc.args.queryLower, tc.args.filterCompleted,
				tc.args.completedAfter, tc.args.completedBefore,
				tc.args.limit)

			gotIDs := make([]string, len(results))
			for i, r := range results {
				gotIDs[i] = r.ID
			}

			if len(gotIDs) != len(tc.wantIDs) {
				t.Fatalf("got IDs %v, want %v", gotIDs, tc.wantIDs)
			}
			wantSet := make(map[string]bool, len(tc.wantIDs))
			for _, id := range tc.wantIDs {
				wantSet[id] = true
			}
			for _, id := range gotIDs {
				if !wantSet[id] {
					t.Errorf("unexpected ID %q in results %v (want %v)", id, gotIDs, tc.wantIDs)
				}
			}
		})
	}
}

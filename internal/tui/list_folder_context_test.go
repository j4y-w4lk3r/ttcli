package tui

import "testing"

func TestListFolderContextForCursor(t *testing.T) {
	m := fixtureModel(120, 40)
	cases := []struct {
		cursor int
		want   string
	}{
		{cursor: indexListRow(m, "X"), want: "X"},
		{cursor: indexListRow(m, "Tech"), want: "X"},
		{cursor: indexListRow(m, "PXC"), want: "X"},
		{cursor: indexListRow(m, "Y"), want: "Y"},
		{cursor: indexListRow(m, "Company"), want: "Y"},
		{cursor: indexListRow(m, "List0"), want: "none"},
	}

	for _, tc := range cases {
		got := m.listFolderContextForCursor(tc.cursor)
		if got != tc.want {
			t.Fatalf("cursor=%d want %q got %q", tc.cursor, tc.want, got)
		}
	}
}

func TestAddListFolderPickerDefault(t *testing.T) {
	m := fixtureModel(120, 40)
	m.paneFocus = paneLists
	m.listCursor = indexListRow(m, "Tech")

	m.openAddListFolderPicker()
	if m.mode != modeListPicker {
		t.Fatalf("mode=%v want list picker", m.mode)
	}
	if m.listPickerPurpose != pickerAddList {
		t.Fatalf("purpose=%v", m.listPickerPurpose)
	}
	row := m.listPickerRows[m.listPickerCursor]
	if row.name != "X" {
		t.Fatalf("default folder=%q want X", row.name)
	}
}

func indexListRow(m model, name string) int {
	for i, r := range m.listRows {
		if r.node.Name == name {
			return i
		}
	}
	return -1
}

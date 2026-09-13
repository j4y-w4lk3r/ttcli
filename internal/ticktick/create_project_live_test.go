package ticktick

import "testing"

func TestCreateProjectInFolderLive(t *testing.T) {
	c, err := New("", CredentialOptions{})
	if err != nil {
		t.Skip(err)
	}
	id, err := c.CreateProject("zz-folder-fix-test", "Y", "", "TASK")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.DeleteProject(id) })
	proj, err := c.GetProject(id)
	if err != nil {
		t.Fatal(err)
	}
	gid, _ := proj["groupId"].(string)
	g, _ := c.ResolveProjectGroup("Y")
	if gid != g.ID {
		t.Fatalf("groupId=%q want %q", gid, g.ID)
	}
}

package process

import "testing"

func TestSanitizedChildEnvironmentRemovesWebGateControlSecrets(t *testing.T) {
	input := []string{
		"PATH=/usr/bin",
		"WEBGATE_AUTHORITY_TOKEN=authority-secret",
		"webgate_admin_token=admin-secret",
		"SERVICE_SETTING=keep-me",
	}
	got := sanitizedChildEnvironment(input)
	want := map[string]bool{"PATH=/usr/bin": true, "SERVICE_SETTING=keep-me": true}
	if len(got) != len(want) {
		t.Fatalf("sanitized environment = %#v", got)
	}
	for _, entry := range got {
		if !want[entry] {
			t.Fatalf("unexpected child environment entry %q", entry)
		}
	}
}

func TestSanitizedChildEnvironmentDropsMalformedEntries(t *testing.T) {
	got := sanitizedChildEnvironment([]string{"MALFORMED", "SAFE=value"})
	if len(got) != 1 || got[0] != "SAFE=value" {
		t.Fatalf("sanitized malformed environment = %#v", got)
	}
}

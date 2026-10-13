package imap

import (
	"testing"

	"github.com/Tahlasandale/monoxide/protonmail"
)

// A draft APPENDed into Drafts stays a draft: no relabelling must happen, or a
// draft a user is still editing would jump to another mailbox mid-edit.
func TestAppendTargetKeepsDrafts(t *testing.T) {
	if got := appendTarget(protonmail.LabelDraft); got != "" {
		t.Errorf("appendTarget(Drafts) = %q, want %q", got, "")
	}
}

// Any other mailbox is a destination the draft has to be moved to, which is
// what makes copying to Sent possible.
func TestAppendTargetRelabelsOtherMailboxes(t *testing.T) {
	for _, label := range []string{
		protonmail.LabelSent,
		protonmail.LabelInbox,
		protonmail.LabelArchive,
		protonmail.LabelAllMail,
		protonmail.LabelTrash,
	} {
		if got := appendTarget(label); got != label {
			t.Errorf("appendTarget(%q) = %q, want %q", label, got, label)
		}
	}
}

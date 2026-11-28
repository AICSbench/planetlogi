package leaderboards

import "testing"

func TestCreatorSubmissionStaysOutOfPublicHubUntilModerated(t *testing.T) {
	svc := NewService(nil)
	const title = "Moon base geometry"
	const summary = "Contact pilot@example.test to discuss this idea."
	entry, err := svc.SubmitCreatorChallenge(t.Context(), "pilot", "Test Astronaut", 4, " "+title+" ", " "+summary+" ")
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID == "" || entry.Status != "PENDING" || entry.Title != title || entry.Summary != summary || entry.AuthorMask != "Test A." {
		t.Fatalf("unexpected submission receipt: %+v", entry)
	}
	for _, locale := range []string{"en", "id"} {
		for _, visible := range svc.GetEventsHub("Asia/Jakarta", locale).CreatorEntries {
			if visible.ID == entry.ID || visible.Title == title || visible.Summary == summary {
				t.Fatalf("unmoderated submission exposed in %s: %+v", locale, visible)
			}
		}
	}
}

func TestMarkNotificationsReadOnlyChangesCurrentStudent(t *testing.T) {
	svc := NewService(nil)
	for _, student := range []string{"pilot", "other-pilot"} {
		items := svc.ListNotifications(student, "en")
		if len(items) == 0 || items[0].Read {
			t.Fatalf("expected an unread notification for %s, got %+v", student, items)
		}
	}
	svc.MarkAllNotificationsRead("pilot")
	items := svc.ListNotifications("pilot", "en")
	if len(items) == 0 {
		t.Fatal("marking notifications as read cleared the list")
	}
	for _, item := range items {
		if !item.Read {
			t.Errorf("notification %s remained unread", item.ID)
		}
	}
	if svc.ListNotifications("other-pilot", "en")[0].Read {
		t.Fatal("marking notifications as read affected another student")
	}
}

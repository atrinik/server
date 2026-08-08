package domain

import "testing"

func TestRecordAndEventOwnImmutablePayloads(t *testing.T) {
	t.Parallel()
	source := []byte("bounded")
	record, err := NewRecord("fixture:one", "fixture", source)
	if err != nil {
		t.Fatal(err)
	}
	event, err := NewEvent(1, "fixture.created", source)
	if err != nil {
		t.Fatal(err)
	}
	source[0] = 'X'
	recordCopy := record.Payload()
	eventCopy := event.Payload()
	if string(recordCopy) != "bounded" || string(eventCopy) != "bounded" {
		t.Fatal("caller mutation reached immutable domain payload")
	}
	recordCopy[0] = 'Y'
	eventCopy[0] = 'Z'
	if string(record.Payload()) != "bounded" || string(event.Payload()) != "bounded" {
		t.Fatal("returned payload exposed internal ownership")
	}
}

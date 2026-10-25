package app

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

func TestInspectionPrefersCompleteFrameRegardlessOfProfileOrder(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			a, c := testApp(t)
			a.captureEnabled = true
			fragment := receiver.DecoderProfile{ID: "fragment", Model: "SyntheticFragment", Bits: 12}
			complete := receiver.DecoderProfile{ID: "complete", Model: "SyntheticComplete", Bits: 16}
			a.config.DecoderProfiles = []receiver.DecoderProfile{fragment, complete}
			if reverse {
				a.config.DecoderProfiles = []receiver.DecoderProfile{complete, fragment}
			}
			before := a.Backup()
			startInspectionCapture(t, a)
			for press := 0; press < 3; press++ {
				c.advance(2 * time.Second)
				// The misleading profile arrives first, as in the reported defect.
				a.Receive([]byte(`{"model":"SyntheticFragment","codes":["{12}a69","{16}ba69","{16}ba69","{16}ba69"]}`))
				if a.learning.Presses != press {
					t.Fatal("fragment counted as a press")
				}
				a.Receive([]byte(`{"model":"SyntheticComplete","codes":["{10}f74","{16}ba69","{16}ba69","{5}28"]}`))
				if a.learning.Presses != press+1 || a.learning.Profile == nil || a.learning.Profile.Bits != 16 {
					t.Fatal("complete frame not selected")
				}
				if press < 2 {
					c.advance(time.Second)
					if _, err := a.ArmLearning(); err != nil {
						t.Fatal(err)
					}
				}
			}
			report, err := a.InspectionResult()
			if err != nil || !strings.Contains(report.Candidate, "{16}ba69") || !report.RepeatedIdentifier {
				t.Fatal("wrong inspection result", err)
			}
			if !reflect.DeepEqual(before, a.Backup()) {
				t.Fatal("inspection changed existing assignments")
			}
		})
	}
}

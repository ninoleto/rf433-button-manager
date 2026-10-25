package app

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestExactConfigured12BitFlexLearnsFragmentsAndRoutesOnce(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	s := a.Settings()
	s.FlexDecoder = "n=Fixture,m=OOK_PWM,s=72,l=272,r=11000,y=1272,bits=12"
	if err := a.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"model":"Fixture","codes":["{1}8","{12}a69","{12}a69","{13}a690"]}`)
	learn(t, a, c, r.Buttons[0].ID, raw)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	configure(t, a, r.Buttons[0].ID, server.URL)
	for i := 0; i < 5; i++ {
		a.Receive(raw)
	}
	a.wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("repeated packets did not produce one webhook", calls.Load())
	}
	a.Receive([]byte(`{"model":"Fixture","codes":["{12}a69","{12}a68"]}`))
	a.wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("conflicting complete row reached webhook")
	}
}

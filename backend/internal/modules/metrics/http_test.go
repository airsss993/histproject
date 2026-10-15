package metrics

import "testing"

func TestValidate(t *testing.T) {
	const visitor = "6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b"
	step := func(n int) *int { return &n }
	for _, test := range []struct {
		name   string
		action Action
		bad    string
	}{
		{"card open", Action{Name: "card_open", EventID: 3, VisitorID: visitor}, ""},
		{"form step", Action{Name: "form_step", Step: step(0), VisitorID: visitor}, ""},
		{"card without event", Action{Name: "card_click", VisitorID: visitor}, "eventId"},
		{"step without number", Action{Name: "form_step", VisitorID: visitor}, "step"},
		{"step too far", Action{Name: "form_step", Step: step(21), VisitorID: visitor}, "step"},
		{"unknown action", Action{Name: "click", EventID: 3, VisitorID: visitor}, "action"},
		{"bad visitor", Action{Name: "card_open", EventID: 3, VisitorID: "x"}, "visitorId"},
	} {
		fields := validate(test.action)
		if test.bad == "" && len(fields) > 0 || test.bad != "" && fields[test.bad] == "" {
			t.Errorf("%s: fields = %v, want error in %q", test.name, fields, test.bad)
		}
	}
}

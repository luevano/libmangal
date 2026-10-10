package metadata

import "testing"

type datedMeta struct {
	fakeMeta
	start, end Date
}

func (m *datedMeta) StartDate() Date { return m.start }
func (m *datedMeta) EndDate() Date   { return m.end }

func TestSeriesJSONPublicationRun(t *testing.T) {
	tests := []struct {
		start, end Date
		want       string
	}{
		{Date{2011, 11, 3}, Date{2016, 7, 1}, "November 2011 - July 2016"},
		{Date{2021, 6, 0}, Date{}, "June 2021 - Present"},
		{Date{2011, 0, 0}, Date{2016, 0, 0}, "2011 - 2016"},
	}
	for _, tt := range tests {
		got := ToSeriesJSON(&datedMeta{start: tt.start, end: tt.end}).PublicationRun
		if got != tt.want {
			t.Errorf("%v - %v: got %q, want %q", tt.start, tt.end, got, tt.want)
		}
	}
}

func TestDateString(t *testing.T) {
	for d, want := range map[Date]string{
		{}:           "",
		{2011, 0, 0}: "2011",
		{2011, 3, 0}: "2011-03",
		{2011, 3, 9}: "2011-03-09",
	} {
		if got := d.String(); got != want {
			t.Errorf("%#v: got %q, want %q", d, got, want)
		}
	}
}
